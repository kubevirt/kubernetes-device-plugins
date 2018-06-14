package random

import (
	"os"
	"strconv"

	"github.com/golang/glog"
	"github.com/kubevirt/device-plugin-manager/pkg/dpm"
	"golang.org/x/net/context"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
)

const (
	hwRandomName      = "hwrng"
	resourceNamespace = "devices.kubevirt.io"
)

func typeToPath(t string) string {
	return "/dev/" + t
}

// Lister is the object responsible for discovering initial pool of devices and their allocation.
type Lister struct{}

type message struct{}

// DevicePlugin is an implementation of DevicePlugin that is capable of exposing /dev/hwrng to containers.
// Note that /dev/random and /dev/urandom are automatically exposed
type DevicePlugin struct {
	randomType string
	counter    int
	devs       []*pluginapi.Device
	update     chan message
}

// GetResourceNamespace TODO: why is this kubevirt.io? this does not seem related to kubevirt
func (Lister) GetResourceNamespace() string {
	return resourceNamespace
}

// Discover discovers hwrng device within the system.
func (Lister) Discover(pluginListCh chan dpm.PluginNameList) {
	var plugins = make(dpm.PluginNameList, 0)

	f, err := os.Open(typeToPath(hwRandomName))
	if err == nil {
		if _, err = f.Read(make([]byte, 1)); err == nil {
			// making sure that the device exists and is readable
			glog.V(3).Infof("Discovered %s", typeToPath(hwRandomName))
			plugins = append(plugins, hwRandomName)
		}
	}
	pluginListCh <- plugins
}

// NewPlugin initializes new device plugin with KVM specific attributes.
func (Lister) NewPlugin(deviceID string) dpm.PluginInterface {
	glog.V(3).Infof("Creating device plugin %s", deviceID)
	return &DevicePlugin{
		randomType: deviceID,
		counter:    0,
		devs:       make([]*pluginapi.Device, 0),
		update:     make(chan message),
	}
}

// ListAndWatch sends gRPC stream of devices.
func (dpi *DevicePlugin) ListAndWatch(e *pluginapi.Empty, s pluginapi.DevicePlugin_ListAndWatchServer) error {
	// Initialize with available device
	dpi.devs = append(dpi.devs, &pluginapi.Device{
		ID:     hwRandomName ,
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
// TODO: many pods can use the same random device. how do we hack around that?
func (dpi *DevicePlugin) Allocate(ctx context.Context, r *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	var response pluginapi.AllocateResponse

	dpi.devs = append(dpi.devs, &pluginapi.Device{
		ID:     dpi.randomType + strconv.Itoa(dpi.counter),
		Health: pluginapi.Healthy,
	})
	// since the hwrng device is unlimited, we allocate a new one 
	// with a new ID every time "Allocate" is called
	dpi.counter++
	
	dpi.update <- message{}

	dev := new(pluginapi.DeviceSpec)
	dev.HostPath = typeToPath(dpi.randomType)
	dev.ContainerPath = typeToPath(dpi.randomType)
	dev.Permissions = "r"
	var devices []*pluginapi.DeviceSpec
	devices = append(devices, dev)
	response.ContainerResponses = append(response.ContainerResponses, &pluginapi.ContainerAllocateResponse{
		Devices: devices})

	return &response, nil
}

// GetDevicePluginOptions returns options to be communicated with Device
// Manager
func (DevicePlugin) GetDevicePluginOptions(context.Context, *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	return nil, nil
}

// PreStartContainer is called, if indicated by Device Plugin during registration phase,
// before each container start. Device plugin can run device specific operations
// such as reseting the device before making devices available to the container
func (DevicePlugin) PreStartContainer(context.Context, *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	return nil, nil
}
