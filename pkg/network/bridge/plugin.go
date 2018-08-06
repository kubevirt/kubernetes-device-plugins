// GO version 1.10 or greater is required. Before that, switching namespaces in
// long running processes in go did not work in a reliable way.

// +build go1.10

package bridge

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"sync"

	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/golang/glog"
	"github.com/kubevirt/kubernetes-device-plugins/pkg/dockerutils"
	"github.com/vishvananda/netlink"
	"golang.org/x/net/context"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
)

const (
	fakeDevicePath      = "/var/run/device-plugin-network-bridge-fakedev"
	nicsPoolSize        = 100
	interfaceNameLen    = 15
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
			glog.V(3).Info("Failed to marshal network interface description due to: %s", err.Error())
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
						glog.V(3).Info("Failed to obtain container's pid, due to: %s", err.Error())
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

func (nbdp *NetworkBridgeDevicePlugin) attachPodToBridge(bridgeIfaceName, contIfaceName string, containerPid int) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	var hostIfaceName string

	contNS, err := ns.GetNS(fmt.Sprintf("/proc/%d/ns/net", containerPid))
	if err != nil {
		return fmt.Errorf("failed to open netns %q: %v", containerPid, err)
	}
	defer contNS.Close()

	err = contNS.Do(func(hostNS ns.NetNS) error {
		hostVeth, _, err := ip.SetupVeth(contIfaceName, 1500, hostNS)
		if err != nil {
			return err
		}
		hostIfaceName = hostVeth.Name
		return nil
	})
	if err != nil {
		return nil
	}

	br, err := netlink.LinkByName(bridgeIfaceName)
	if err != nil {
		return fmt.Errorf("failed to lookup %q: %v", bridgeIfaceName, err)
	}

	hostVeth, err := netlink.LinkByName(hostIfaceName)
	if err != nil {
		return fmt.Errorf("failed to lookup %q: %v", hostIfaceName, err)
	}

	if err := netlink.LinkSetMaster(hostVeth, br.(*netlink.Bridge)); err != nil {
		return fmt.Errorf("failed to connect %q to bridge %v: %v", hostIfaceName, bridgeIfaceName, err)
	}

	return nil
}

func randString(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}
