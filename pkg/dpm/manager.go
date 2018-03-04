package dpm

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/golang/glog"

	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1alpha"
)

const (
	startPluginServerRetries   = 3
	startPluginServerRetryWait = 3 * time.Second
)

// Manager contains the main machinery of this framework. It uses user defined lister to monitor
// available resources and start/stop plugins accordingly. It also handles system signals and
// unexpected kubelet events.
type Manager struct {
	lister ListerInterface
}

// NewManager is the canonical way of initializing Manager. User must provide ListerInterface
// implementation. Lister will provide information about handled resources, monitor their
// availability and provide method to spawn plugins that will handle found resources.
func NewManager(lister ListerInterface) *Manager {
	dpm := &Manager{
		lister: lister,
	}
	return dpm
}

// Run starts the Manager. It sets up the infrastructure and handles system signals, Kubelet socket
// watch and monitoring of available resources as well as starting and stoping of plugins.
func (dpm *Manager) Run() {
	glog.V(3).Info("Starting device plugin manager")

	// First important signal channel is the os signal channel. We only care about (somewhat) small
	// subset of available signals.
	glog.V(3).Info("Registering for system signal notifications")
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT)

	// The other important channel is filesystem notification channel, responsible for watching
	// device plugin directory.
	glog.V(3).Info("Registering for notifications of filesystem changes in device plugin directory")
	fsWatcher, _ := fsnotify.NewWatcher()
	defer fsWatcher.Close()
	fsWatcher.Add(pluginapi.DevicePluginPath)

	// Create list of running plugins and start Discover method of given lister. This method is
	// responsible of notifying manager about changes in available plugins.
	var pluginsMap = make(map[string]devicePlugin)
	glog.V(3).Info("Starting Discovery on new plugins")
	pluginsCh := make(chan ResourceLastNamesList)
	defer close(pluginsCh)
	go dpm.lister.Discover(pluginsCh)

	// Finally start a loop that will handle messages from opened channels.
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
				// TODO: Kubelet doesn't really clean-up it's socket, so this is currently
				// manual-testing thing. Could we solve Kubelet deaths better?
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

func (dpm *Manager) handleNewPlugins(currentPluginsMap map[string]devicePlugin, newPluginsList ResourceLastNamesList) {
	var wg sync.WaitGroup

	newPluginsSet := make(map[string]bool)
	for _, newPluginLastName := range newPluginsList {
		newPluginsSet[newPluginLastName] = true
	}

	// Add new plugins
	for newPluginLastName, _ := range newPluginsSet {
		wg.Add(1)
		go func() {
			if _, ok := currentPluginsMap[newPluginLastName]; !ok {
				glog.V(3).Infof("Adding a new plugin \"%s\"", newPluginLastName)
				plugin := newDevicePlugin(dpm.lister.GetResourceNamespace(), newPluginLastName, dpm.lister.NewPlugin(newPluginLastName))
				startUpPlugin(newPluginLastName, plugin)
				currentPluginsMap[newPluginLastName] = plugin
			}
			wg.Done()
		}()
	}
	wg.Wait()

	// Remove old plugins
	for pluginLastName, plugin := range currentPluginsMap {
		wg.Add(1)
		go func() {
			if _, ok := newPluginsSet[pluginLastName]; !ok {
				shutDownPlugin(pluginLastName, plugin)
				delete(currentPluginsMap, pluginLastName)
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

func (dpm *Manager) startPluginServers(pluginsMap map[string]devicePlugin) {
	var wg sync.WaitGroup

	for pluginLastName, plugin := range pluginsMap {
		wg.Add(1)
		go func() {
			startPluginServer(pluginLastName, plugin)
			wg.Done()
		}()
	}
	wg.Wait()
}

func (dpm *Manager) stopPluginServers(pluginsMap map[string]devicePlugin) {
	var wg sync.WaitGroup

	for pluginLastName, plugin := range pluginsMap {
		wg.Add(1)
		go func() {
			stopPluginServer(pluginLastName, plugin)
		}()
	}
	wg.Wait()
}

func (dpm *Manager) shutDownPlugins(pluginsMap map[string]devicePlugin) {
	var wg sync.WaitGroup

	for pluginLastName, plugin := range pluginsMap {
		wg.Add(1)
		go func() {
			shutDownPlugin(pluginLastName, plugin)
			delete(pluginsMap, pluginLastName)
			wg.Done()
		}()
	}
	wg.Wait()
}

func startUpPlugin(pluginLastName string, plugin devicePlugin) {
	if devicePluginImplementation, ok := plugin.DevicePlugin.(PluginInterfaceStart); ok {
		err := devicePluginImplementation.Start()
		if err != nil {
			glog.Errorf("Failed to start plugin \"%s\": %s", pluginLastName, err)
		}
	}
	startPluginServer(pluginLastName, plugin)
}

func shutDownPlugin(pluginLastName string, plugin devicePlugin) {
	stopPluginServer(pluginLastName, plugin)
	if devicePluginImplementation, ok := plugin.DevicePlugin.(PluginInterfaceStop); ok {
		err := devicePluginImplementation.Stop()
		if err != nil {
			glog.Errorf("Failed to stop plugin \"%s\": %s", pluginLastName, err)
		}
	}
}

func startPluginServer(pluginLastName string, plugin devicePlugin) {
	for i := 1; i <= startPluginServerRetries; i++ {
		err := plugin.StartServer()
		if err == nil {
			return
		} else if i == startPluginServerRetries {
			glog.V(3).Infof("Failed to start plugin server \"%s\", within given tries: %s", pluginLastName, err)
		} else {
			glog.Errorf("Failed to start plugin server \"%s\", waiting %d before next try: %s", pluginLastName, startPluginServerRetryWait, err)
			time.Sleep(startPluginServerRetryWait)
		}
	}
}

func stopPluginServer(pluginLastName string, plugin devicePlugin) {
	err := plugin.StopServer()
	if err != nil {
		glog.Errorf("Failed to stop plugin \"%s\": %s", pluginLastName, err)
	}
}
