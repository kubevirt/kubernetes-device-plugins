package dpm

type DeviceMap map[string][]string

type DeviceLister interface {
	Discover() *DeviceMap
	NewDevicePlugin(string, []string) DevicePlugin
}
