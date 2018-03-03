package dpm

type PluginList []string

type PluginListerInterface interface {
	GetResourceName() string
	Discover(chan PluginList)
	NewDevicePlugin(string) DevicePluginImplementationInterface
}
