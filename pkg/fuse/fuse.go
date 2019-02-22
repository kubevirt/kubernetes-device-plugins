package fuse

import (
	"os"
	"context"
	"github.com/golang/glog"
	"github.com/kubevirt/device-plugin-manager/pkg/dpm"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
	"fmt"
	"time"
	"os/exec"
)

const (
	FUSEPath 		 = "/dev/fuse"
	FUSEName  		= "fuse"
	Namespace 		= "devices.kubevirt.io"
	fusePoolSize 	= 128
)

type message struct{}

type FusePlugin struct {
	counter int
}

// object responsible for discovering initial pool of devices and their allocation.
type FuseLister struct{}

func (l FuseLister) GetResourceNamespace() string {
	return Namespace
}

// Discovery discovers the FUSE device within the system.
func (l FuseLister) Discover(pluginListCh chan dpm.PluginNameList) {
	var plugins = make(dpm.PluginNameList, 0)

	if _, err := os.Stat(FUSEPath); err == nil {
		glog.V(3).Infof("Discovered %s", FUSEPath)
		plugins = append(plugins, FUSEName)
	}
	pluginListCh <- plugins
}

func (FuseLister) NewPlugin(deviceID string) dpm.PluginInterface {
	glog.V(3).Infof("Creating device plugin %s", deviceID)

	return &FusePlugin{}
}

// Ensure the fuse module is loaded
func (p *FusePlugin) Start() error {
	glog.V(3).Infof("Ensuring fuse kernel module is loaded")
	cmd := exec.Command("modprobe", "fuse")
	err := cmd.Run()

	return err
}

func (p *FusePlugin) ListAndWatch(e *pluginapi.Empty, s pluginapi.DevicePlugin_ListAndWatchServer) error {
	// initialize devices
	devs := p.generateDevices()

	glog.V(3).Infof("Returning %d devices", len(devs))

	s.Send(&pluginapi.ListAndWatchResponse{Devices: devs})

	for {
		time.Sleep(60 * time.Second)
	}
}

func (p *FusePlugin) generateDevices() []*pluginapi.Device {
	var devs []*pluginapi.Device
	for i := 0; i < fusePoolSize; i++ {
		devs = append(devs, &pluginapi.Device{
			ID: fmt.Sprintf("%s-%02d", FUSEName, i),
			Health: pluginapi.Healthy,
		})
	}
	return devs
}

func (p *FusePlugin) Allocate(ctx context.Context, request *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	var response pluginapi.AllocateResponse
	var car pluginapi.ContainerAllocateResponse
	var dev *pluginapi.DeviceSpec

	glog.V(3).Infof("Allocated virtual fuse device %d (requested id: %s)", p.counter, request.ContainerRequests[0].DevicesIDs[0])
	p.counter += 1

	car = pluginapi.ContainerAllocateResponse{}

	dev = new(pluginapi.DeviceSpec)
	dev.HostPath = FUSEPath
	dev.ContainerPath = FUSEPath
	dev.Permissions = "rw"
	car.Devices = append(car.Devices, dev)

	response.ContainerResponses = append(response.ContainerResponses, &car)

	return &response, nil
}

// GetDevicePluginOptions returns options to be communicated with Device
// Manager
func (p *FusePlugin) GetDevicePluginOptions(context.Context, *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	return nil, nil
}

// PreStartContainer is called, if indicated by Device Plugin during registration phase,
// before each container start. Device plugin can run device specific operations
// such as resetting the device before making devices available to the container
func (p *FusePlugin) PreStartContainer(context.Context, *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	return nil, nil
}
