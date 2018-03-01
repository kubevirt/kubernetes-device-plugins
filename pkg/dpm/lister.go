package dpm

type PluginList []string

type PluginLister interface {
	Discover() *PluginList
	NewDevicePlugin(string) DevicePluginInterface
}
