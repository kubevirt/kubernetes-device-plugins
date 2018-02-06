package dpm

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/fsnotify/fsnotify"
	"github.com/golang/glog"

	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1alpha"
)

type DevicePluginManager struct {
	plugins   []DevicePluginInterface
	lister    DeviceLister
	stopCh    chan struct{}
	signalCh  chan os.Signal
	fsWatcher *fsnotify.Watcher
}

func NewDevicePluginManager(lister DeviceLister) *DevicePluginManager {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT)
	stopCh := make(chan struct{})

	fsWatcher, _ := fsnotify.NewWatcher()
	fsWatcher.Add(pluginapi.DevicePluginPath)

	dpm := &DevicePluginManager{
		stopCh:    stopCh,
		signalCh:  sigs,
		fsWatcher: fsWatcher,
	}

	go dpm.handleSignals()

	devices := lister.Discover()

	for deviceClass, deviceIDs := range *devices {
		dpm.plugins = append(dpm.plugins, lister.NewDevicePlugin(deviceClass, deviceIDs))
	}

	return dpm
}

func (dpm *DevicePluginManager) Run() {
	defer dpm.fsWatcher.Close()
	dpm.startPlugins()

	<-dpm.stopCh
	dpm.stopPlugins()
}

func (dpm *DevicePluginManager) startPlugins() {
	for _, plugin := range dpm.plugins {
		go plugin.Start()
	}
}

func (dpm *DevicePluginManager) stopPlugins() {
	for _, plugin := range dpm.plugins {
		plugin.Stop()
	}
}

func (dpm *DevicePluginManager) handleSignals() {
	for {
		select {
		case s := <-dpm.signalCh:
			switch s {
			case syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT:
				glog.V(3).Infof("Received signal \"%v\", shutting down", s)
				close(dpm.stopCh)
			}
		case event := <-dpm.fsWatcher.Events:
			if event.Name == pluginapi.KubeletSocket {
				if event.Op&fsnotify.Create == fsnotify.Create {
					dpm.startPlugins()
				}
				if event.Op&fsnotify.Remove == fsnotify.Remove {
					dpm.stopPlugins()
				}
			}
		}
	}
}
