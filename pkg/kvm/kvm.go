package kvm

import (
	"os"
	"strconv"

	"github.com/golang/glog"
	"golang.org/x/net/context"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1alpha"

	"kubevirt.io/kubernetes-device-plugins/pkg/dpm"
)

const (
	KVMPath           = "/dev/kvm"
	KVMName           = "kvm"
	resourceNamespace = "devices.kubevirt.io"
)

// KVMLister is the object responsible for discovering initial pool of devices and their allocation.
type KVMLister struct{}

type message struct{}

// KVMDevicePlugin is an implementation of DevicePlugin that is capable of exposing /dev/kvm to containers.
type KVMDevicePlugin struct {
	counter int
	devs    []*pluginapi.Device
	update  chan message
}

func (kvm KVMLister) GetResourceNamespace() string {
	return resourceNamespace
}

// Discovery discovers all KVM devices within the system.
func (kvm KVMLister) Discover(pluginListCh chan dpm.ResourceLastNamesList) {
	var plugins = make(dpm.ResourceLastNamesList, 0)

	if _, err := os.Stat(KVMPath); err == nil {
		glog.V(3).Infof("Discovered %s", KVMPath)
		plugins = append(plugins, "kvm")
	}

	pluginListCh <- plugins
}

// NewPlugin initializes new device plugin with KVM specific attributes.
func (kvm KVMLister) NewPlugin(deviceID string) dpm.PluginInterface {
	glog.V(3).Infof("Creating device plugin %s", deviceID)
	return &KVMDevicePlugin{
		counter: 0,
		devs:    make([]*pluginapi.Device, 0),
		update:  make(chan message),
	}
}

// ListAndWatch sends gRPC stream of devices.
func (dpi *KVMDevicePlugin) ListAndWatch(e *pluginapi.Empty, s pluginapi.DevicePlugin_ListAndWatchServer) error {
	// Initialize with one available device
	dpi.devs = append(dpi.devs, &pluginapi.Device{
		ID:     KVMName + strconv.Itoa(dpi.counter),
		Health: pluginapi.Healthy,
	})

	s.Send(&pluginapi.ListAndWatchResponse{Devices: dpi.devs})

	for {
		select {
		case <-dpi.update:
			s.Send(&pluginapi.ListAndWatchResponse{Devices: dpi.devs})
		}
	}
}

// Allocate allocates a set of devices to be used by container runtime environment.
func (dpi *KVMDevicePlugin) Allocate(ctx context.Context, r *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	var response pluginapi.AllocateResponse

	// This is super-hacky workaround for device quantity (that we don't care about in this case).
	dpi.devs = append(dpi.devs, &pluginapi.Device{
		ID:     KVMName + strconv.Itoa(dpi.counter),
		Health: pluginapi.Healthy,
	})
	dpi.counter += 1
	dpi.update <- message{}

	dev := new(pluginapi.DeviceSpec)
	dev.HostPath = "/dev/kvm"
	dev.ContainerPath = "/dev/kvm"
	dev.Permissions = "rw"
	response.Devices = append(response.Devices, dev)

	return &response, nil
}
