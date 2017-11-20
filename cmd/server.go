/*
 * Copyright (c) 2017 Martin Polednik
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE.
 */

package main

import (
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"path"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/golang/glog"
	"github.com/mpolednik/linux-vfio-k8s-dpi/pkg"
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
		iommuGroup, err := pci.GetIOMMUGroup(id)
		if err != nil {
			return &response, err
		}
		vfioPath := pci.ConstructVFIOPath(iommuGroup)
		glog.V(3).Infof("Got request to allocate device %s from IOMMU group %d (path %s)", id, iommuGroup, vfioPath)
		dev := new(pluginapi.DeviceSpec)
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

func main() {
	flag.Parse()

	var plugins []*DevicePlugin
	devices := pci.Discover()

	for deviceClass, deviceIDs := range *devices {
		plugins = append(plugins, NewDevicePlugin(deviceClass, deviceIDs))
	}

	for _, plugin := range plugins {
		go plugin.Serve()
	}

	watcher, _ := fsnotify.NewWatcher()
	defer watcher.Close()
	watcher.Add(pluginapi.DevicePluginPath)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

L:
	for {
		select {
		case err := <-watcher.Errors:
			log.Printf("inotify: %s", err)
		case s := <-sigs:
			switch s {
			default:
				log.Printf("Received signal \"%v\", shutting down", s)

				for _, plugin := range plugins {
					plugin.Stop()
				}
				break L
			}
		}
	}
}
