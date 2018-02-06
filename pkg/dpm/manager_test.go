package dpm

import (
	"syscall"
	"testing"

	"github.com/golang/mock/gomock"
)

func TestDevicePluginManagerFlow(t *testing.T) {
	devices := &DeviceMap{
		"a": []string{"a", "b", "c"},
	}

	ctrl := gomock.NewController(t)
	lister := NewMockDeviceLister(ctrl)
	plugin := NewMockDevicePluginInterface(ctrl)

	lister.EXPECT().Discover().Return(devices)
	lister.EXPECT().NewDevicePlugin("a", []string{"a", "b", "c"}).Return(DevicePluginInterface(plugin))
	plugin.EXPECT().Start()
	plugin.EXPECT().Stop()

	manager := NewDevicePluginManager(lister)

	syscall.Kill(syscall.Getpid(), syscall.SIGINT)
	manager.Run()
}
