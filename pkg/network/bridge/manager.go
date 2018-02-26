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

func (bl BridgeLister) Discover() *dpm.DeviceMap {
	bridgesListRaw := os.Getenv(BridgesListEnvironmentVariable)
	bridges := strings.Split(bridgesListRaw, ",")

	var devices = make(dpm.DeviceMap)

	for _, bridgeName := range bridges {
		if len(bridgeName) > maxBridgeNameLength {
			glog.Fatalf("Bridge name (%s) cannot be longer than %d characters", bridgeName, maxBridgeNameLength)
		}
		devices[bridgeName] = devices[bridgeName]
	}

	return &devices
}

func (bl BridgeLister) NewDevicePlugin(bridge string, nics []string) dpm.DevicePluginInterface {
	return dpm.DevicePluginInterface(newDevicePlugin(bridge, nics))
}
