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
	"os"
	"os/signal"
	"syscall"

	"github.com/fsnotify/fsnotify"
	"github.com/golang/glog"
	"github.com/mpolednik/linux-vfio-k8s-dpi/pkg/pci"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1alpha"
)

type DevicePluginManager struct {
	plugins []*pci.DevicePlugin
	stopCh  chan struct{}
}

func NewDevicePluginManager(stopCh chan struct{}) *DevicePluginManager {
	dpm := &DevicePluginManager{
		stopCh: stopCh,
	}

	// We need a pci.DevicePlugin for *each* "device class" (fancy name for vendor:device tuple).
	// That is a limitation of DPI architecture.
	devices := pci.Discover()

	for deviceClass, deviceIDs := range *devices {
		dpm.plugins = append(dpm.plugins, pci.NewDevicePlugin(deviceClass, deviceIDs))
	}

	return dpm
}

func (dpm *DevicePluginManager) Run() {
	for _, plugin := range dpm.plugins {
		go plugin.Serve()
	}

	<-dpm.stopCh
	dpm.stop()
}

func (dpm *DevicePluginManager) stop() {
	for _, plugin := range dpm.plugins {
		plugin.Stop()
	}
}

func handleSignals(stopCh chan struct{}, errors chan error, sigs chan os.Signal) {
	for {
		select {
		case err := <-errors:
			glog.V(3).Infof("inotify: %s", err)
		case s := <-sigs:
			switch s {
			case syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT:
				glog.V(3).Infof("Received signal \"%v\", shutting down", s)
				close(stopCh)
			}
		}
	}
}

func main() {
	flag.Parse()

	// Let's start by making sure vfio_pci module is loaded. Without that, binds/unbinds will fail.
	// Small caveat: the loaded module is called vfio_pci, but when it's being probed the name to use is vfio-pci!
	if !pci.IsModuleLoaded("vfio_pci") {
		err := pci.LoadModule("vfio-pci")
		// If we were not able to load the module, we're out of luck.
		if err != nil {
			os.Exit(1)
		}
	}

	// Watch for changes in the sockets directory.
	watcher, _ := fsnotify.NewWatcher()
	defer watcher.Close()
	watcher.Add(pluginapi.DevicePluginPath)

	// And setup a signal handler.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT)
	stopCh := make(chan struct{})

	go handleSignals(stopCh, watcher.Errors, sigs)

	dpm := NewDevicePluginManager(stopCh)
	dpm.Run()
}
