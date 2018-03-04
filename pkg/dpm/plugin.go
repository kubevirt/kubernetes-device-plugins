package dpm

import (
	"net"
	"os"
	"path"
	"sync"
	"time"

	"github.com/golang/glog"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1alpha"
)

// PluginInterface is a mandatory interface that must be implemented by all plugins. It is
// identical to DevicePluginServer interface of device plugin API. In version v1alpha this
// interface contains methods Allocate and ListAndWatch. For more information see
// https://godoc.org/k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1alpha#DevicePluginServer
type PluginInterface interface {
	pluginapi.DevicePluginServer
}

// PluginInterfaceStart is an optional interface that could be implemented by plugin. If case Start
// is implemented, it will be executed by Manager after plugin instantiation and before its
// registartion to kubelet. This method could be used to prepare resources before they are offered
// to Kubernetes.
type PluginInterfaceStart interface {
	Start() error
}

// PluginInterfaceStop is an optional interface that could be implemented by plugin. If case Stop
// is implemented, it will be executed by Manager after the plugin is unregistered from kubelet.
// This method could be used to tear down resources.
type PluginInterfaceStop interface {
	Stop() error
}

// DevicePlugin represents a gRPC server client/server.
type devicePlugin struct {
	DevicePlugin PluginInterface
	ResourceName string
	Socket       string
	Server       *grpc.Server
	Running      bool
	Starting     sync.Mutex
}

func newDevicePlugin(resourceName string, deviceName string, devicePluginImplementation PluginInterface) devicePlugin {
	return devicePlugin{
		DevicePlugin: devicePluginImplementation,
		Socket:       pluginapi.DevicePluginPath + resourceName + "_" + deviceName,
		ResourceName: resourceName + "/" + deviceName,
	}
}

// StartServer starts the gRPC server and registers the device plugin to Kubelet. Calling
// StartServer on started object is NOOP.
func (dpi *devicePlugin) StartServer() error {
	glog.V(3).Info("Starting plugin server")

	// If Kubelet socket is created, we may try to start the same plugin concurrently. To avoid
	// that, let's make plugins startup a critical section.
	dpi.Starting.Lock()
	defer dpi.Starting.Unlock()

	// If we've acquired the lock after waiting for the Start to finish, we don't need to do
	// anything (as long as the plugin is running).
	if dpi.Running {
		return nil
	}

	err := dpi.serve()
	if err != nil {
		return err
	}

	dpi.Running = true

	err = dpi.register(pluginapi.KubeletSocket, dpi.ResourceName)
	if err != nil {
		dpi.StopServer()
		return err
	}

	return nil
}

// serve starts the gRPC server of the device plugin.
func (dpi *devicePlugin) serve() error {
	glog.V(3).Info("Starting the DPI gRPC server")

	err := dpi.cleanup()
	if err != nil {
		glog.Errorf("Failed to setup a DPI gRPC server: %s", err)
		return err
	}

	sock, err := net.Listen("unix", dpi.Socket)
	if err != nil {
		glog.Errorf("Failed to setup a DPI gRPC server: %s", err)
		return err
	}

	dpi.Server = grpc.NewServer([]grpc.ServerOption{}...)
	pluginapi.RegisterDevicePluginServer(dpi.Server, dpi.DevicePlugin)

	go dpi.Server.Serve(sock)
	glog.V(3).Info("Serving requests...")
	// Wait till grpc server is ready.
	for i := 0; i < 10; i++ {
		services := dpi.Server.GetServiceInfo()
		if len(services) > 1 {
			break
		}
		time.Sleep(1 * time.Second)
	}

	return nil
}

// register registers the device plugin (as gRPC client call) for the given resourceName with
// Kubelet DPI infrastructure.
func (dpi *devicePlugin) register(kubeletEndpoint, resourceName string) error {
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
	glog.Infof("Registration for endpoint %s resourceName %s", path.Base(dpi.Socket), resourceName)
	reqt := &pluginapi.RegisterRequest{
		Version:      pluginapi.Version,
		Endpoint:     path.Base(dpi.Socket),
		ResourceName: resourceName,
	}

	_, err = client.Register(context.Background(), reqt)
	if err != nil {
		glog.Errorf("Registration failed: %s", err)
		glog.Errorf("Make sure that the DevicePlugins feature gate is enabled")
		return err
	}
	return nil
}

// StopServer stops the gRPC server. Trying to stop already stopped plugin emits an info-level
// log message.
func (dpi *devicePlugin) StopServer() error {
	glog.V(3).Info("Stopping plugin server")

	if !dpi.Running {
		glog.V(3).Info("Tried to stop stopped DPI")
		return nil
	}

	glog.V(3).Info("Stopping the DPI gRPC server")
	dpi.Server.Stop()
	dpi.Running = false

	return dpi.cleanup()
}

// cleanup is a helper to remove DPI's socket.
func (dpi *devicePlugin) cleanup() error {
	if err := os.Remove(dpi.Socket); err != nil && !os.IsNotExist(err) {
		glog.Errorf("Could not clean up socket %s: %s", dpi.Socket, err)
		return err
	}

	return nil
}
