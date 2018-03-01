package bridge

import (
	"os"
	"strings"

	"github.com/golang/glog"
	"kubevirt.io/kubernetes-device-plugins/pkg/dpm"
)

const (
	BridgesListEnvironmentVariable = "BRIDGES"
	// maximal interface name length (15) - nic index suffix (3)
	maxBridgeNameLength = 12
)

type BridgeLister struct{}

func (bl BridgeLister) Discover() *dpm.PluginList {
	var plugins = make(dpm.PluginList, 0)

	bridgesListRaw := os.Getenv(BridgesListEnvironmentVariable)
	bridges := strings.Split(bridgesListRaw, ",")

	for _, bridgeName := range bridges {
		if len(bridgeName) > maxBridgeNameLength {
			glog.Fatalf("Bridge name (%s) cannot be longer than %d characters", bridgeName, maxBridgeNameLength)
		}
		plugins = append(plugins, bridgeName)
	}

	return &plugins
}

func (bl BridgeLister) NewDevicePlugin(bridgeName string) dpm.DevicePluginInterface {
	return dpm.DevicePluginInterface(newDevicePlugin(bridgeName))
}
