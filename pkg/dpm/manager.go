package dpm

import (
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/fsnotify/fsnotify"
	"github.com/golang/glog"

	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1alpha"
)

// Manager is container for 1 or more DevicePlugins.
type Manager struct {
	lister ListerInterface
}

// NewManager is the canonical way of initializing Manager. It sets up the infrastructure for
// the manager to correctly handle signals and Kubelet socket watch.
func NewManager(lister ListerInterface) *Manager {
	dpm := &Manager{
		lister: lister,
	}
	return dpm
}

// Run starts the Manager.
func (dpm *Manager) Run() {
	glog.V(3).Info("Starting device plugin manager")

	var pluginsMap = make(map[string]devicePlugin)

	// First important signal channel is the os signal channel. We only care about (somewhat) small subset of available signals.
	glog.V(3).Info("Registering for system signal notifications")
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT)

	// The other important channel is filesystem notification channel, responsible for watching device plugin directory.
	glog.V(3).Info("Registering for notifications of filesystem changes in device plugin directory")
	fsWatcher, _ := fsnotify.NewWatcher()
	defer fsWatcher.Close()
	fsWatcher.Add(pluginapi.DevicePluginPath)

	glog.V(3).Info("Starting Discovery on new plugins")
	pluginsCh := make(chan PluginNamesList)
	defer close(pluginsCh)
	go dpm.lister.Discover(pluginsCh)

	glog.V(3).Info("Handling incoming signals")
HandleSignals:
	for {
		select {
		case newPluginsList := <-pluginsCh:
			glog.V(3).Infof("Received new list of plugins: %s", newPluginsList)
			dpm.handleNewPlugins(pluginsMap, newPluginsList)
		case event := <-fsWatcher.Events:
			if event.Name == pluginapi.KubeletSocket {
				glog.V(3).Infof("Received kubelet socket event: %s", event)
				if event.Op&fsnotify.Create == fsnotify.Create {
					dpm.startPluginServers(pluginsMap)
				}
				// TODO: Kubelet doesn't really clean-up it's socket, so this is currently manual-testing thing. Could we solve Kubelet deaths better?
				if event.Op&fsnotify.Remove == fsnotify.Remove {
					dpm.stopPluginServers(pluginsMap)
				}
			}
		case s := <-signalCh:
			switch s {
			case syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT:
				glog.V(3).Infof("Received signal \"%v\", shutting down", s)
				dpm.shutDownPlugins(pluginsMap)
				break HandleSignals
			}
		}
	}
}

func (dpm *Manager) handleNewPlugins(currentPluginsMap map[string]devicePlugin, newPluginsList PluginNamesList) {
	var wg sync.WaitGroup

	newPluginsSet := make(map[string]bool)
	for _, newPluginName := range newPluginsList {
		newPluginsSet[newPluginName] = true
	}

	// add new
	for newPluginName, _ := range newPluginsSet {
		wg.Add(1)
		go func() {
			if _, ok := currentPluginsMap[newPluginName]; !ok {
				glog.V(3).Infof("Adding a new plugin \"%s\"", newPluginName)
				plugin := newDevicePlugin(dpm.lister.GetResourceName(), newPluginName, dpm.lister.NewPlugin(newPluginName))
				startUpPlugin(newPluginName, plugin)
				currentPluginsMap[newPluginName] = plugin
			}
			wg.Done()
		}()
	}
	wg.Wait()

	// remove old
	for pluginName, plugin := range currentPluginsMap {
		wg.Add(1)
		go func() {
			if _, ok := newPluginsSet[pluginName]; !ok {
				shutDownPlugin(pluginName, plugin)
				delete(currentPluginsMap, pluginName)
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

func (dpm *Manager) startPluginServers(pluginsMap map[string]devicePlugin) {
	var wg sync.WaitGroup

	for pluginName, plugin := range pluginsMap {
		wg.Add(1)
		go func() {
			err := plugin.StartServer()
			if err != nil {
				glog.Errorf("Failed to start plugin server \"%s\": %s", pluginName, err)
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

func (dpm *Manager) stopPluginServers(pluginsMap map[string]devicePlugin) {
	var wg sync.WaitGroup

	for pluginName, plugin := range pluginsMap {
		wg.Add(1)
		go func() {
			err := plugin.StopServer()
			if err != nil {
				glog.Errorf("Failed to stop plugin server \"%s\": %s", pluginName, err)
			}
			wg.Done()
		}()
	}
	wg.Wait()
}
func (dpm *Manager) shutDownPlugins(pluginsMap map[string]devicePlugin) {
	var wg sync.WaitGroup

	for pluginName, plugin := range pluginsMap {
		wg.Add(1)
		go func() {
			shutDownPlugin(pluginName, plugin)
			delete(pluginsMap, pluginName)
		}()
	}
	wg.Wait()
}

func startUpPlugin(pluginName string, plugin devicePlugin) {
	if devicePluginImplementation, ok := plugin.DevicePlugin.(PluginInterfaceStart); ok {
		err := devicePluginImplementation.Start()
		if err != nil {
			glog.Errorf("Failed to start plugin \"%s\": %s", pluginName, err)
		}
	}
	err := plugin.StartServer()
	if err != nil {
		glog.Errorf("Failed to start plugin server \"%s\": %s", pluginName, err)
	}
}

func shutDownPlugin(pluginName string, plugin devicePlugin) {
	err := plugin.StopServer()
	if err != nil {
		glog.Errorf("Failed to stop plugin \"%s\": %s", pluginName, err)
	}
	if devicePluginImplementation, ok := plugin.DevicePlugin.(PluginInterfaceStop); ok {
		err := devicePluginImplementation.Stop()
		if err != nil {
			glog.Errorf("Failed to stop plugin \"%s\": %s", pluginName, err)
		}
	}
}
