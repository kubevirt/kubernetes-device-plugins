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
	lister PluginLister
}

// NewDevicePluginManager is the canonical way of initializing DevicePluginManager. It sets up the infrastructure for
// the manager to correctly handle signals and Kubelet socket watch. TODO: not anymore
func NewDevicePluginManager(lister PluginLister) *DevicePluginManager {
	dpm := &DevicePluginManager{
		lister: lister,
	}
	return dpm
}

// Run starts the DevicePluginManager.
func (dpm *DevicePluginManager) Run() {
	var pluginsMap = make(map[string]DevicePluginInterface)

	// First important signal channel is the os signal channel. We only care about (somewhat) small subset of available signals.
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT)

	// The other important channel is filesystem notification channel, responsible for watching device plugin directory.
	fsWatcher, _ := fsnotify.NewWatcher()
	defer fsWatcher.Close()
	fsWatcher.Add(pluginapi.DevicePluginPath)

	pluginsCh := make(chan PluginList)
	defer close(pluginsCh)
	go dpm.lister.Discover(pluginsCh)

HandleSignals:
	for {
		select {
		case newPlugins := <-pluginsCh:
			newPluginsSet := make(map[string]bool)
			for _, newPluginName := range newPlugins {
				newPluginsSet[newPluginName] = true
			}
			// add new
			for newPluginName, _ := range newPluginsSet {
				if _, ok := pluginsMap[newPluginName]; !ok {
					plugin := dpm.lister.NewDevicePlugin(newPluginName)
					go plugin.Start()
					pluginsMap[newPluginName] = plugin
				}
			}
			// remove old
			for pluginName, plugin := range pluginsMap {
				if _, ok := newPluginsSet[pluginName]; !ok {
					plugin.Stop()
					delete(pluginsMap, pluginName)
				}
			}
		case event := <-fsWatcher.Events:
			if event.Name == pluginapi.KubeletSocket {
				if event.Op&fsnotify.Create == fsnotify.Create {
					for _, plugin := range pluginsMap {
						plugin.Start()
					}
				}
				// TODO: Kubelet doesn't really clean-up it's socket, so this is currently manual-testing thing. Could we solve Kubelet deaths better?
				if event.Op&fsnotify.Remove == fsnotify.Remove {
					for _, plugin := range pluginsMap {
						plugin.Stop()
					}
				}
			}
		case s := <-signalCh:
			switch s {
			case syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT:
				glog.V(3).Infof("Received signal \"%v\", shutting down", s)
				for _, plugin := range pluginsMap {
					plugin.Stop()
				}
				break HandleSignals
			}
		}
	}
}
