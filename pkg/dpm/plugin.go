package dpm

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

type Message struct{}

type DevicePluginInterface interface {
	pluginapi.DevicePluginServer
	Run() error
	Stop() error
}

// DevicePlugin represents a gRPC server client/server.
type DevicePlugin struct {
	Devs         []*pluginapi.Device
	Socket       string
	StopCh       chan interface{}
	Server       *grpc.Server
	ResourceName string
	Deps         DevicePluginInterface
	Update       chan Message
	Running      bool
}

// start starts the gRPC server of the device plugin.
func (dpi *DevicePlugin) start() error {
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
	pluginapi.RegisterDevicePluginServer(dpi.Server, dpi.Deps)

	go dpi.Server.Serve(sock)
	dpi.Running = true
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

// stop stops the gRPC server
func (dpi *DevicePlugin) Stop() error {
	if !dpi.Running {
		return nil
	}

	glog.V(3).Info("Stopping the DPI gRPC server")
	dpi.Server.Stop()
	close(dpi.StopCh)
	dpi.Running = false

	return dpi.cleanup()
}

// register registers the device plugin (as gRPC client call) for the given resourceName with Kubelet DPI infrastructure.
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
	glog.Infof("Registration for endpoint %s resourceName %s", path.Base(dpi.Socket), resourceName)
	reqt := &pluginapi.RegisterRequest{
		Version:      pluginapi.Version,
		Endpoint:     path.Base(dpi.Socket),
		ResourceName: resourceName,
	}

	_, err = client.Register(context.Background(), reqt)
	if err != nil {
		glog.Errorf("Registration failed: %s", err)
		return err
	}
	return nil
}

// Serve starts the gRPC server and registers the device plugin to Kubelet
func (dpi *DevicePlugin) Run() error {
	err := dpi.start()
	if err != nil {
		return err
	}

	err = dpi.register(pluginapi.KubeletSocket, dpi.ResourceName)
	if err != nil {
		dpi.Stop()
		return err
	}

	return nil
}

// cleanup is a helper to remove DPI's socket.
func (dpi *DevicePlugin) cleanup() error {
	if err := os.Remove(dpi.Socket); err != nil && !os.IsNotExist(err) {
		glog.Errorf("Could not clean up socket %s: %s", dpi.Socket, err)
		return err
	}

	return nil
}
