package dpm

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/golang/glog"
)

type DevicePluginManager struct {
	plugins  []DevicePluginInterface
	lister   DeviceLister
	stopCh   chan struct{}
	signalCh chan os.Signal
}

func NewDevicePluginManager(lister DeviceLister) *DevicePluginManager {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT)
	stopCh := make(chan struct{})

	dpm := &DevicePluginManager{
		stopCh:   stopCh,
		signalCh: sigs,
	}

	go dpm.handleSignals()

	devices := lister.Discover()

	for deviceClass, deviceIDs := range *devices {
		dpm.plugins = append(dpm.plugins, lister.NewDevicePlugin(deviceClass, deviceIDs))
	}

	return dpm
}

func (dpm *DevicePluginManager) Run() {
	for _, plugin := range dpm.plugins {
		go plugin.Start()
	}

	<-dpm.stopCh
	dpm.stop()
}

func (dpm *DevicePluginManager) stop() {
	for _, plugin := range dpm.plugins {
		plugin.Stop()
	}
}

func (dpm *DevicePluginManager) handleSignals() {
	for {
		select {
		case s := <-dpm.signalCh:
			switch s {
			case syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT:
				glog.V(3).Infof("Received signal \"%v\", shutting down", s)
				close(dpm.stopCh)
			}
		}
	}
}
