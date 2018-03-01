package dpm

import (
	"syscall"
	"testing"

	"github.com/golang/mock/gomock"
)

func TestDevicePluginManagerFlow(t *testing.T) {
	devices := &PluginList{"a"}

	ctrl := gomock.NewController(t)
	lister := NewMockPluginLister(ctrl)
	plugin := NewMockDevicePluginInterface(ctrl)

	lister.EXPECT().Discover().Return(devices)
	lister.EXPECT().NewDevicePlugin("a").Return(DevicePluginInterface(plugin))
	plugin.EXPECT().Start()
	plugin.EXPECT().Stop()

	manager := NewDevicePluginManager(lister)

	syscall.Kill(syscall.Getpid(), syscall.SIGINT)
	manager.Run()
}
