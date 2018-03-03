package dpm

type PluginList []string

type PluginLister interface {
	Discover(chan PluginList)
	NewDevicePlugin(string) DevicePluginInterface
}
