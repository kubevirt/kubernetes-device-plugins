package bridge

import (
	"container/list"
	"fmt"
	"math/rand"
	"os/exec"
	"time"

	"github.com/golang/glog"
	"github.com/vishvananda/netlink"
	"golang.org/x/net/context"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1alpha"
	"kubevirt.io/kubernetes-device-plugins/pkg/dockerutils"
	"kubevirt.io/kubernetes-device-plugins/pkg/dpm"
)

const (
	resourceNamespace   = "bridge.network.kubevirt.io/"
	fakeDevicePath      = "/tmp/deviceplugin-network-bridge-fakedev"
	interfaceNameLen    = 15
	interfaceNamePrefix = "nic_"
	letterBytes         = "abcdefghijklmnopqrstuvwxyz0123456789"
)

type NetworkBridgeDevicePlugin struct {
	dpm.DevicePlugin
	bridge       string
	assignmentCh chan *Assignment
}

type Assignment struct {
	DeviceID      string
	ContainerPath string
}

func newDevicePlugin(bridge string, nics []string) *NetworkBridgeDevicePlugin {
	if !bridgeExists(bridge) {
		glog.Exitf("Bridge %s does not exist", bridge)
	}

	var devs []*pluginapi.Device

	for _, nic := range nics {
		devs = append(devs, &pluginapi.Device{
			ID:     nic,
			Health: pluginapi.Healthy,
		})
	}

	glog.V(3).Infof("Creating device plugin %s, initial devices %v", bridge, nics)
	ret := &NetworkBridgeDevicePlugin{
		dpm.DevicePlugin{
			Socket:       pluginapi.DevicePluginPath + bridge,
			Devs:         devs,
			ResourceName: resourceNamespace + bridge,
			Update:       make(chan dpm.Message),
		},
		bridge,
		make(chan *Assignment),
	}
	ret.DevicePlugin.Deps = ret

	// TODO: This should be triggered by start()
	createFakeDevice()
	go ret.attachPods()

	return ret
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

func createFakeDevice() {
	glog.V(3).Info("Creating fake block device")
	cmd := exec.Command("mknod", fakeDevicePath, "b", "1", "1")
	cmd.Run()
}

func (nbdp *NetworkBridgeDevicePlugin) ListAndWatch(e *pluginapi.Empty, s pluginapi.DevicePlugin_ListAndWatchServer) error {
	var devs []*pluginapi.Device

	for _, d := range nbdp.DevicePlugin.Devs {
		devs = append(devs, &pluginapi.Device{
			ID:     d.ID,
			Health: pluginapi.Healthy,
		})
	}

	s.Send(&pluginapi.ListAndWatchResponse{Devices: devs})

	for {
		select {
		case <-nbdp.DevicePlugin.Update:
			s.Send(&pluginapi.ListAndWatchResponse{Devices: nbdp.DevicePlugin.Devs})
		}
	}
}

func (nbdp *NetworkBridgeDevicePlugin) Allocate(ctx context.Context, r *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	var response pluginapi.AllocateResponse

	for _, nic := range r.DevicesIDs {
		dev := new(pluginapi.DeviceSpec)
		assignmentPath := getAssignmentPath(nbdp.bridge, nic)
		dev.HostPath = fakeDevicePath
		dev.ContainerPath = assignmentPath
		dev.Permissions = "r"
		response.Devices = append(response.Devices, dev)
		nbdp.assignmentCh <- &Assignment{
			nic,
			assignmentPath,
		}
	}

	return &response, nil
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

			err = attachPodToBridge(nbdp.bridge, containerPid)
			if err == nil {
				glog.V(3).Info("Successfully attached pod to a bridge")
			} else {
				glog.V(3).Infof("Pod attachment failed with: %s", err)
			}
			pendingAssignments.Remove(a)
		}
	}
}

func attachPodToBridge(bridgeName string, containerPid int) error {
	linkName := randInterfaceName()
	peerName := randInterfaceName()

	bridge, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return err
	}

	link := &netlink.Veth{LinkAttrs: netlink.LinkAttrs{Name: linkName, MasterIndex: bridge.Attrs().Index}, PeerName: peerName}

	err = netlink.LinkAdd(link)
	if err != nil {
		return err
	}

	peer, err := netlink.LinkByName(peerName)
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
