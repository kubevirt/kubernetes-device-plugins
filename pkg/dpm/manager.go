package dpm

type DevicePluginManager struct {
	plugins []DevicePlugin
	lister  DeviceLister
	stopCh  chan struct{}
}

func NewDevicePluginManager(stopCh chan struct{}, lister DeviceLister) *DevicePluginManager {
	dpm := &DevicePluginManager{
		stopCh: stopCh,
	}

	devices := lister.Discover()

	for deviceClass, deviceIDs := range *devices {
		dpm.plugins = append(dpm.plugins, lister.NewDevicePlugin(deviceClass, deviceIDs))
	}

	return dpm
}

func (dpm *DevicePluginManager) Run() {
	for _, plugin := range dpm.plugins {
		go plugin.Run()
	}

	<-dpm.stopCh
	dpm.stop()
}

func (dpm *DevicePluginManager) stop() {
	for _, plugin := range dpm.plugins {
		plugin.Stop()
	}
}
