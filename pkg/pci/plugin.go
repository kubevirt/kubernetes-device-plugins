package pci

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/golang/glog"
	"golang.org/x/net/context"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1alpha"

	"kubevirt.io/kubernetes-device-plugins/pkg/dpm"
)

const (
	resourceNamespace = "devices.kubevirt.io/"
)

// DevicePlugin represents a gRPC server client/server.
type VFIODevicePlugin struct {
	dpm.DevicePlugin
	vendorID string
}

var iommuMutex = &sync.Mutex{}

// newDevicePlugin creates a DevicePlugin for specific deviceID, using deviceIDs as initial device "pool".
func newDevicePlugin(vendorID string) *VFIODevicePlugin {
	glog.V(3).Infof("Creating device plugin %s", vendorID)
	ret := &VFIODevicePlugin{
		dpm.DevicePlugin{
			Socket:       pluginapi.DevicePluginPath + vendorID,
			ResourceName: resourceNamespace + vendorID,
		},
		vendorID,
	}
	ret.DevicePlugin.Deps = ret

	return ret
}

// ListAndWatch lists devices.
func (dpi *VFIODevicePlugin) ListAndWatch(e *pluginapi.Empty, s pluginapi.DevicePlugin_ListAndWatchServer) error {
	var devs []*pluginapi.Device

	filepath.Walk("/sys/bus/pci/devices", func(path string, info os.FileInfo, err error) error {
		// TODO: fix logs
		glog.V(3).Infof("Discovering device in %s", path)
		if info.IsDir() {
			glog.V(3).Infof("Not a device, continuing")
			return nil
		}

		vendorID, deviceID, err := getDeviceVendor(info.Name())
		if err != nil {
			glog.V(3).Infof("Could not process device %s", info.Name())
			return filepath.SkipDir
		}

		if vendorID == dpi.vendorID {
			deviceClass := formatDeviceID(vendorID, deviceID)
			devs = append(devs, &pluginapi.Device{
				ID:     deviceClass,
				Health: pluginapi.Healthy,
			})
		}

		return nil
	})

	s.Send(&pluginapi.ListAndWatchResponse{Devices: devs})

	for {
		select {}
	}
}

// formatDeviceID formats the device class so that the pods can request it in the limits.
// Typically, vendor:device would be used, but the resource name may not contain ":".
func formatDeviceID(vendorID string, deviceID string) string {
	return strings.Join([]string{vendorID, deviceID}, "_")
}

// Allocate allocates a set of devices to be used by container runtime environment.
func (dpi *VFIODevicePlugin) Allocate(ctx context.Context, r *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	var response pluginapi.AllocateResponse

	// The big IOMMU allocation mutex. Because the plugins run in concurrent fashion, and each
	// plugin manages different device class, we need to ensure that IOMMU groups are not violated.
	// Imagine devices 0000:00:01.0 from 1000:1000 and device 0000:00:02.0 from 2000:2000.
	// If two containers were started, each requiring different device, and these devices were
	// within a single IOMMU group, both containers may not start up.
	iommuMutex.Lock()
	defer iommuMutex.Unlock()

	for _, id := range r.DevicesIDs {
		iommuGroup, err := getIOMMUGroup(id)
		if err != nil {
			return &response, err
		}

		vfioPath := constructVFIOPath(iommuGroup)

		err = overrideIOMMUGroup(iommuGroup, "vfio-pci")
		if err != nil {
			return &response, err
		}

		err = unbindIOMMUGroup(iommuGroup)
		if err != nil {
			return &response, err
		}

		err = probeIOMMUGroup(iommuGroup)
		if err != nil {
			return &response, err
		}

		dev := new(pluginapi.DeviceSpec)
		dev.HostPath = vfioPath
		dev.ContainerPath = vfioPath
		dev.Permissions = "rw"
		response.Devices = append(response.Devices, dev)
	}

	// As somewhat special case, we also have to make sure that the pod has /dev/vfio/vfio path.
	// TODO: make sure that having len(response.Devices) > len(r.DeviceIDs) is OK with DPI.
	dev := new(pluginapi.DeviceSpec)
	dev.HostPath = "/dev/vfio/vfio"
	dev.ContainerPath = "/dev/vfio/vfio"
	dev.Permissions = "rw"
	response.Devices = append(response.Devices, dev)

	return &response, nil
}
