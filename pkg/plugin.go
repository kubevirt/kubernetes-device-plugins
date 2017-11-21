package pci

import (
	"net"
	"os"
	"path"
	"time"

	"github.com/golang/glog"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1alpha"
)

const (
	resourceNamespace = "mpolednik.github.io/"
)

type DevicePlugin struct {
	devs         []*pluginapi.Device
	socket       string
	stop         chan interface{}
	server       *grpc.Server
	resourceName string
}

func NewDevicePlugin(deviceID string, deviceIDs []string) *DevicePlugin {
	var devs []*pluginapi.Device

	for _, deviceID := range deviceIDs {
		devs = append(devs, &pluginapi.Device{
			ID:     deviceID,
			Health: pluginapi.Healthy,
		})
	}

	glog.V(3).Infof("Creating device plugin %s, initial devices %v", deviceID, devs)
	return &DevicePlugin{
		socket:       pluginapi.DevicePluginPath + deviceID,
		devs:         devs,
		resourceName: resourceNamespace + deviceID,
		stop:         make(chan interface{}),
	}
}

// Start starts the gRPC server of the device plugin
func (dpi *DevicePlugin) start() error {
	glog.V(3).Info("Starting the DPI gRPC server")

	err := dpi.cleanup()
	if err != nil {
		glog.Errorf("Failed to setup a DPI gRPC server: %s", err)
		return err
	}

	sock, err := net.Listen("unix", dpi.socket)
	if err != nil {
		glog.Errorf("Failed to setup a DPI gRPC server: %s", err)
		return err
	}

	dpi.server = grpc.NewServer([]grpc.ServerOption{}...)
	pluginapi.RegisterDevicePluginServer(dpi.server, dpi)

	go dpi.server.Serve(sock)
	glog.V(3).Info("Serving requests...")
	// Wait till grpc server is ready.
	for i := 0; i < 10; i++ {
		services := dpi.server.GetServiceInfo()
		if len(services) > 1 {
			break
		}
		time.Sleep(1 * time.Second)
	}

	return nil
}

// Stop stops the gRPC server
func (dpi *DevicePlugin) Stop() error {
	glog.V(3).Info("Stopping the DPI gRPC server")
	dpi.server.Stop()
	close(dpi.stop)

	return dpi.cleanup()
}

// Register registers the device plugin for the given resourceName with Kubelet.
func (dpi *DevicePlugin) register(kubeletEndpoint, resourceName string) error {
	glog.V(3).Info("Registering the DPI with Kubelet")

	conn, err := grpc.Dial(kubeletEndpoint, grpc.WithInsecure(),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}))
	defer conn.Close()
	if err != nil {
		glog.Errorf("Could not dial gRPC: %s", err)
		return err
	}
	client := pluginapi.NewRegistrationClient(conn)
	glog.Infof("Registration for endpoint %s resourceName %s", path.Base(dpi.socket), resourceName)
	reqt := &pluginapi.RegisterRequest{
		Version:      pluginapi.Version,
		Endpoint:     path.Base(dpi.socket),
		ResourceName: resourceName,
	}

	_, err = client.Register(context.Background(), reqt)
	if err != nil {
		glog.Errorf("Registration failed: %s", err)
		return err
	}
	return nil
}

// ListAndWatch lists devices and update that list according to the Update call
func (dpi *DevicePlugin) ListAndWatch(e *pluginapi.Empty, s pluginapi.DevicePlugin_ListAndWatchServer) error {
	var devs []*pluginapi.Device

	for _, d := range dpi.devs {
		devs = append(devs, &pluginapi.Device{
			ID:     d.ID,
			Health: pluginapi.Healthy,
		})
	}

	s.Send(&pluginapi.ListAndWatchResponse{Devices: devs})

	for {
		select {
		case <-dpi.stop:
			return nil
		}
	}
}

func (dpi *DevicePlugin) Allocate(ctx context.Context, r *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	var response pluginapi.AllocateResponse

	for _, id := range r.DevicesIDs {
		iommuGroup, err := getIOMMUGroup(id)
		if err != nil {
			return &response, err
		}

		vfioPath := constructVFIOPath(iommuGroup)

		unbindIOMMUGroup(iommuGroup)

		err = bindIOMMUGroup(iommuGroup, "vfio-pci")
		if err != nil {
			return &response, err
		}

		dev := new(pluginapi.DeviceSpec)
		dev.HostPath = vfioPath
		dev.ContainerPath = vfioPath
		dev.Permissions = "rw"
		response.Devices = append(response.Devices, dev)
	}

	return &response, nil
}

// Serve starts the gRPC server and register the device plugin to Kubelet
func (dpi *DevicePlugin) Serve() error {
	err := dpi.start()
	if err != nil {
		return err
	}

	err = dpi.register(pluginapi.KubeletSocket, dpi.resourceName)
	if err != nil {
		dpi.Stop()
		return err
	}

	return nil
}

func (dpi *DevicePlugin) cleanup() error {
	if err := os.Remove(dpi.socket); err != nil && !os.IsNotExist(err) {
		glog.Errorf("Could not clean up socket %s: %s", dpi.socket, err)
		return err
	}

	return nil
}
