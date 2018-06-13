package bridge

import (
	"container/list"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"time"

	"github.com/golang/glog"
	"github.com/vishvananda/netlink"
	"golang.org/x/net/context"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
	"kubevirt.io/kubernetes-device-plugins/pkg/dockerutils"
)

const (
	fakeDevicePath      = "/var/run/device-plugin-network-bridge-fakedev"
	nicsPoolSize        = 100
	interfaceNameLen    = 15
	interfaceNamePrefix = "nic_"
	letterBytes         = "abcdefghijklmnopqrstuvwxyz0123456789"
	assignmentTimeout   = 30 * time.Minute
)

type NetworkBridgeDevicePlugin struct {
	bridge       string
	assignmentCh chan *Assignment
}

type Assignment struct {
	DeviceID      string
	ContainerPath string
	Created       time.Time
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
		for _, nic := range req.DevicesIDs {
			dev := new(pluginapi.DeviceSpec)
			assignmentPath := getAssignmentPath(nbdp.bridge, nic)
			dev.HostPath = fakeDevicePath
			dev.ContainerPath = assignmentPath
			dev.Permissions = "r"
			devices = append(devices, dev)

			nbdp.assignmentCh <- &Assignment{
				nic,
				assignmentPath,
				time.Now(),
			}
		}
		response.ContainerResponses = append(response.ContainerResponses, &pluginapi.ContainerAllocateResponse{
			Devices: devices})
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
	pendingAssignments := list.New()

	cli, err := dockerutils.NewClient()
	if err != nil {
		glog.V(3).Info("Failed to connect to Docker")
		panic(err)
	}

	for {
		select {
		case assignment := <-nbdp.assignmentCh:
			glog.V(3).Infof("Received a new assignment: %s", assignment)
			pendingAssignments.PushBack(assignment)
		default:
			time.Sleep(time.Second)
		}

		for a := pendingAssignments.Front(); a != nil; a = a.Next() {
			assignment := a.Value.(*Assignment)
			glog.V(3).Infof("Handling pending assignment for: %s", assignment.DeviceID)

			if time.Now().After(assignment.Created.Add(assignmentTimeout)) {
				glog.V(3).Info("Assignment timed out")
				pendingAssignments.Remove(a)
				continue
			}

			containerID, err := cli.GetContainerIDByMountedDevice(assignment.ContainerPath)
			if err != nil {
				glog.V(3).Info("Container was not found")
				continue
			}

			containerPid, err := cli.GetPidByContainerID(containerID)
			if err != nil {
				glog.V(3).Info("Failed to obtain container's pid")
				continue
			}

			err = attachPodToBridge(nbdp.bridge, assignment.DeviceID, containerPid)
			if err == nil {
				glog.V(3).Info("Successfully attached pod to a bridge")
			} else {
				glog.V(3).Infof("Pod attachment failed with: %s", err)
			}
			pendingAssignments.Remove(a)
		}
	}
}

func attachPodToBridge(bridgeName, nicName string, containerPid int) error {
	linkName := randInterfaceName()

	bridge, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return err
	}

	link := &netlink.Veth{LinkAttrs: netlink.LinkAttrs{Name: linkName, MasterIndex: bridge.Attrs().Index}, PeerName: nicName}

	err = netlink.LinkAdd(link)
	if err != nil {
		return err
	}

	peer, err := netlink.LinkByName(nicName)
	if err != nil {
		netlink.LinkDel(link)
		return err
	}

	err = netlink.LinkSetNsPid(peer, containerPid)
	if err != nil {
		netlink.LinkDel(link)
		return err
	}

	return nil
}

func randInterfaceName() string {
	b := make([]byte, interfaceNameLen-len(interfaceNamePrefix))
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return interfaceNamePrefix + string(b)
}
