package video

import (
	"os"
	"path/filepath"

	"github.com/golang/glog"
	"github.com/kubevirt/device-plugin-manager/pkg/dpm"
	"golang.org/x/net/context"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
)

const (
	resourceNamespace = "devices.kubevirt.io"
)

// Lister is the object responsible for discovering initial pool of devices and their allocation.
type Lister struct{}

type message struct{}

// DevicePlugin is an implementation of DevicePlugin that is capable of exposing /dev/video* to containers.
type DevicePlugin struct {
	name       string
	devs       []*pluginapi.Device
	update     chan message
}

// GetResourceNamespace TODO: why is this kubevirt.io? this does not seem related to kubevirt
func (Lister) GetResourceNamespace() string {
	return resourceNamespace
}

// Discover discovers video device within the system.
func (Lister) Discover(pluginListCh chan dpm.PluginNameList) {
	var plugins = make(dpm.PluginNameList, 0)

	_ = filepath.Walk("/dev", func(path string, info os.FileInfo, err error) error {
        if err != nil {
			glog.Errorf("Failure accessing a path %q: %v\n", path, err)
            return err 
        }
        if ! info.IsDir() {
            isVideo,_ := filepath.Match("video[0-9]", info.Name())
            if isVideo {
				//glog.V(3).Infof("Discovered %s", info.Name())
				glog.Errorf("Discovered %s", info.Name())
				plugins = append(plugins, info.Name())
			} 
        }
        return nil 
    })  
	pluginListCh <- plugins
}

// NewPlugin initializes new device plugin.
func (Lister) NewPlugin(deviceID string) dpm.PluginInterface {
	//glog.V(3).Infof("Creating device plugin %s", deviceID)
	glog.Errorf("Creating device plugin %s", deviceID)
	return &DevicePlugin{
		name: deviceID,
		devs:       make([]*pluginapi.Device, 0),
		update:     make(chan message),
	}
}

// ListAndWatch sends gRPC stream of devices.
func (dpi *DevicePlugin) ListAndWatch(e *pluginapi.Empty, s pluginapi.DevicePlugin_ListAndWatchServer) error {
	// Initialize with available device
	dpi.devs = append(dpi.devs, &pluginapi.Device{
		ID:     dpi.name,
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

func nameToPath(name string) string {
	return "/dev/" + name
}

// Allocate allocates a set of devices to be used by container runtime environment.
func (dpi *DevicePlugin) Allocate(ctx context.Context, r *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	var response pluginapi.AllocateResponse

	dpi.devs = append(dpi.devs, &pluginapi.Device{
		ID:     dpi.name,
		Health: pluginapi.Healthy,
	})
	
	dpi.update <- message{}

	dev := new(pluginapi.DeviceSpec)
	dev.HostPath = nameToPath(dpi.name)
	dev.ContainerPath = nameToPath(dpi.name)
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
