package dpm

type DeviceMap map[string][]string

// DeviceLister is the interface that plugin-specific lister needs to implement in order to register with DevicePluginManager.
type DeviceLister interface {
	Discover() *DeviceMap
	NewDevicePlugin(string, []string) DevicePluginInterface
}
