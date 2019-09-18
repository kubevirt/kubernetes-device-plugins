package bridge

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"time"

	"sync"

	"github.com/golang/glog"
	"github.com/kubevirt/kubernetes-device-plugins/pkg/dockerutils"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/net/context"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
)

const (
	fakeDevicePath      = "/var/run/device-plugin-network-bridge-fakedev"
	nicsPoolSize        = 100
	interfaceNameLen    = 15
	interfaceNamePrefix = "nic_"
	letterBytes         = "abcdefghijklmnopqrstuvwxyz0123456789"
	attachmentRetries   = 60
	protocolEthernet    = "Ethernet"
	envVarNamePrefix    = "NETWORK_INTERFACE_RESOURCES_"
	envVarNameSuffixLen = 8
)

type NetworkBridgeDevicePlugin struct {
	bridge       string
	assignmentCh chan *Assignment
}

type Assignment struct {
	DeviceID      string
	ContainerPath string
}

type vnic struct {
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
}

type networkInterfaceResources struct {
	Name       string `json:"name"`
	Interfaces []vnic `json:"interfaces"`
}

func (nbdp *NetworkBridgeDevicePlugin) Start() error {
	err := createFakeDevice()
	if err != nil {
		glog.Exitf("Failed to create fake device: %s", err)
	}

	go nbdp.attachPods()

	return nil
}

func createFakeDevice() error {
	_, stat_err := os.Stat(fakeDevicePath)
	if stat_err == nil {
		glog.V(3).Info("Fake block device already exists")
		return nil
	} else if os.IsNotExist(stat_err) {
		glog.V(3).Info("Creating fake block device")
		cmd := exec.Command("mknod", fakeDevicePath, "b", "1", "1")
		err := cmd.Run()
		return err
	} else {
		panic(stat_err)
	}
}

func (nbdp *NetworkBridgeDevicePlugin) ListAndWatch(e *pluginapi.Empty, s pluginapi.DevicePlugin_ListAndWatchServer) error {
	bridgeDevs := nbdp.generateBridgeDevices()
	noBridgeDevs := make([]*pluginapi.Device, 0)
	emitResponse := func(bridgeExists bool) {
		if bridgeExists {
			glog.V(3).Info("Bridge exists, sending ListAndWatch reponse with available ports")
			s.Send(&pluginapi.ListAndWatchResponse{Devices: bridgeDevs})
		} else {
			glog.V(3).Info("Bridge does not exist, sending ListAndWatch reponse with no ports")
			s.Send(&pluginapi.ListAndWatchResponse{Devices: noBridgeDevs})
		}
	}

	didBridgeExist := bridgeExists(nbdp.bridge)
	emitResponse(didBridgeExist)

	for {
		doesBridgeExist := bridgeExists(nbdp.bridge)
		if didBridgeExist != doesBridgeExist {
			emitResponse(doesBridgeExist)
			didBridgeExist = doesBridgeExist
		}
		time.Sleep(10 * time.Second)
	}
}

func (nbdp *NetworkBridgeDevicePlugin) generateBridgeDevices() []*pluginapi.Device {
	var bridgeDevs []*pluginapi.Device
	for i := 0; i < nicsPoolSize; i++ {
		bridgeDevs = append(bridgeDevs, &pluginapi.Device{
			ID:     fmt.Sprintf("%s-%02d", nbdp.bridge, i),
			Health: pluginapi.Healthy,
		})
	}
	return bridgeDevs
}

func bridgeExists(bridge string) bool {
	link, err := netlink.LinkByName(bridge)
	if err != nil {
		return false
	}
	if _, ok := link.(*netlink.Bridge); ok {
		return true
	} else {
		return false
	}
}

func (nbdp *NetworkBridgeDevicePlugin) Allocate(ctx context.Context, r *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	var response pluginapi.AllocateResponse

	for _, req := range r.ContainerRequests {
		var devices []*pluginapi.DeviceSpec
		var vnics []vnic
		for _, nic := range req.DevicesIDs {
			dev := new(pluginapi.DeviceSpec)
			assignmentPath := getAssignmentPath(nbdp.bridge, nic)
			dev.HostPath = fakeDevicePath
			dev.ContainerPath = assignmentPath
			dev.Permissions = "r"
			devices = append(devices, dev)
			vnics = append(vnics, vnic{nic, protocolEthernet})

			nbdp.assignmentCh <- &Assignment{
				nic,
				assignmentPath,
			}
		}

		vnicsPerInterface := networkInterfaceResources{
			Name:       fmt.Sprintf("%s/%s", resourceNamespace, nbdp.bridge),
			Interfaces: vnics,
		}

		envVarName := fmt.Sprintf("%s%s", envVarNamePrefix, strings.ToUpper(randString(envVarNameSuffixLen)))
		marshalled, err := json.Marshal(vnicsPerInterface)
		if err != nil {
			glog.V(3).Infof("Failed to marshal network interface description due to: %s", err.Error())
			continue
		}

		envs := map[string]string{
			envVarName: string(marshalled),
		}

		response.ContainerResponses = append(response.ContainerResponses, &pluginapi.ContainerAllocateResponse{
			Devices: devices, Envs: envs,
		})

	}

	return &response, nil
}

// GetDevicePluginOptions returns options to be communicated with Device
// Manager
func (NetworkBridgeDevicePlugin) GetDevicePluginOptions(context.Context, *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	return nil, nil
}

// PreStartContainer is called, if indicated by Device Plugin during registeration phase,
// before each container start. Device plugin can run device specific operations
// such as reseting the device before making devices available to the container
func (NetworkBridgeDevicePlugin) PreStartContainer(context.Context, *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	return nil, nil
}

func getAssignmentPath(bridge string, nic string) string {
	return fmt.Sprintf("/tmp/device-plugin-network-bridge/%s/%s", bridge, nic)
}

func (nbdp *NetworkBridgeDevicePlugin) attachPods() {
	cli, err := dockerutils.NewClient()
	if err != nil {
		glog.V(3).Info("Failed to connect to Docker")
		panic(err)
	}

	var attachMutex sync.Mutex

	for {
		go func(assignment *Assignment) {
			glog.V(3).Infof("Received a new assignment: %s", assignment)
			for i := 1; i <= attachmentRetries; i++ {
				if attached := func() bool {
					containerID, err := cli.GetContainerIDByMountedDevice(assignment.ContainerPath)
					if err != nil {
						glog.V(3).Infof("Container was not found, due to: %s", err.Error())
						return false
					}

					containerPid, err := cli.GetPidByContainerID(containerID)
					if err != nil {
						glog.V(3).Infof("Failed to obtain container's pid, due to: %s", err.Error())
						return false
					}

					attachMutex.Lock()
					err = nbdp.attachPodToBridge(nbdp.bridge, assignment.DeviceID, containerPid)
					attachMutex.Unlock()

					if err == nil {
						glog.V(3).Infof("Successfully attached pod to a bridge: %s", nbdp.bridge)
						return true
					}

					glog.V(3).Infof("Pod attachment failed with: %s", err.Error())
					return false
				}(); attached {
					break
				}
				time.Sleep(time.Duration(i) * time.Second)
			}
		}(<-nbdp.assignmentCh)
	}
}

func (nbdp *NetworkBridgeDevicePlugin) attachPodToBridge(bridgeName, nicName string, containerPid int) error {
	linkName := randInterfaceName()

	// fetch the bridge, this is expected to succeed since attachPodToBridge is invoked only if bridge exists
	bridge, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return err
	}

	// create the virtual interface that should be connected to the bridge
	link := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name:        linkName,
			MasterIndex: bridge.Attrs().Index,
			MTU:         bridge.Attrs().MTU,
			NetNsID:     bridge.Attrs().NetNsID},
		PeerName: nicName}

	err = netlink.LinkAdd(link)
	if err != nil {
		return err
	}

	// set interface up
	err = netlink.LinkSetUp(link)
	if err != nil {
		netlink.LinkDel(link)
		return err
	}

	// get the peer interface
	peer, err := netlink.LinkByName(nicName)
	if err != nil {
		netlink.LinkDel(link)
		return err
	}

	// add peer to pod namespace
	err = netlink.LinkSetNsPid(peer, containerPid)
	if err != nil {
		netlink.LinkDel(link)
		return err
	}

	// store current namespace
	originalNS, err := netns.Get()
	if err != nil {
		netlink.LinkDel(link)
		return err
	}

	// get namespace of the pod
	ns, err := netns.GetFromPid(containerPid)
	if err != nil {
		netlink.LinkDel(link)
		return err
	}

	// set to pod namespace so that interface values could be set
	err = netns.Set(ns)
	if err != nil {
		netlink.LinkDel(link)
		return err
	}

	// set back to the original namespace before we leave this function
	defer func() {
		setErr := netns.Set(originalNS)
		if setErr != nil {
			// if we cannot go back the the original namespace
			// the plugin cannot be used anymore and a restart is needed
			panic(setErr)
		}
	}()

	// get the peer back, now from the new namespace
	peer, err = netlink.LinkByName(nicName)
	if err != nil {
		netlink.LinkDel(link)
		return err
	}

	// set MTU on the peer
	err = netlink.LinkSetMTU(peer, link.MTU)
	if err != nil {
		netlink.LinkDel(link)
		return err
	}

	// set peer interface up
	err = netlink.LinkSetUp(peer)
	if err != nil {
		netlink.LinkDel(link)
		return err
	}

	return nil
}

func randInterfaceName() string {
	suffixLength := interfaceNameLen - len(interfaceNamePrefix)
	return interfaceNamePrefix + randString(suffixLength)
}

func randString(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}
