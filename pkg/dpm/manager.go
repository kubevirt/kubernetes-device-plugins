package dpm

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/fsnotify/fsnotify"
	"github.com/golang/glog"

	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1alpha"
)

// DevicePluginManager is container for 1 or more DevicePlugins.
type DevicePluginManager struct {
	plugins   []DevicePluginInterface
	lister    PluginLister
	stopCh    chan struct{}
	signalCh  chan os.Signal
	fsWatcher *fsnotify.Watcher
}

// NewDevicePluginManager is the canonical way of initializing DevicePluginManager. It sets up the infrastructure for
// the manager to correctly handle signals and Kubelet socket watch.
func NewDevicePluginManager(lister PluginLister) *DevicePluginManager {
	stopCh := make(chan struct{})

	// First important signal channel is the os signal channel. We only care about (somewhat) small subset of available signals.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT)

	// The other important channel is filesystem notification channel, responsible for watching device plugin directory.
	fsWatcher, _ := fsnotify.NewWatcher()
	fsWatcher.Add(pluginapi.DevicePluginPath)

	dpm := &DevicePluginManager{
		stopCh:    stopCh,
		signalCh:  sigs,
		fsWatcher: fsWatcher,
	}

	go dpm.handleSignals()

	// We can now move to functionality: first, the initial list of plugins
	plugins := lister.Discover()

	// As we use the pool to initialize plugins (the actual gRPC servers) themselves.
	for _, pluginName := range *plugins {
		dpm.plugins = append(dpm.plugins, lister.NewDevicePlugin(pluginName))
	}

	return dpm
}

// Run starts the DevicePluginManager.
func (dpm *DevicePluginManager) Run() {
	defer dpm.fsWatcher.Close()
	dpm.startPlugins()

	// The manager cannot be restarted, it is expected to be the `core` of an application.
	<-dpm.stopCh
	dpm.stopPlugins()
}

// startPlugins is helper function to start the underlying gRPC device plugins.
func (dpm *DevicePluginManager) startPlugins() {
	for _, plugin := range dpm.plugins {
		go plugin.Start()
	}
}

// startPlugins is helper function to stop the underlying gRPC device plugins.
func (dpm *DevicePluginManager) stopPlugins() {
	for _, plugin := range dpm.plugins {
		plugin.Stop()
	}
}

// handleSignals is the main signal handler that is responsible for handling of OS signals and Kubelet device plugins directory changes.
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
				// TODO: Kubelet doesn't really clean-up it's socket, so this is currently manual-testing thing. Could we solve Kubelet deaths better?
				if event.Op&fsnotify.Remove == fsnotify.Remove {
					dpm.stopPlugins()
				}
			}
		}
	}
}
