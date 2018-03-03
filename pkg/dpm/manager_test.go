package dpm

import (
	"syscall"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
)

func TestDevicePluginManagerFlow(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	lister := NewMockPluginLister(ctrl)
	plugin := NewMockDevicePluginInterface(ctrl)

	lister.EXPECT().
		Discover(gomock.Any()).
		Do(func(pluginListCh chan PluginList) {
			pluginListCh <- PluginList{"a"}
		})
	lister.EXPECT().NewDevicePlugin("a").Return(DevicePluginInterface(plugin))
	plugin.EXPECT().Start()
	plugin.EXPECT().Stop()

	manager := NewDevicePluginManager(lister)
	go func() {
		time.Sleep(100 * time.Millisecond)
		syscall.Kill(syscall.Getpid(), syscall.SIGINT)
	}()
	manager.Run()
}
