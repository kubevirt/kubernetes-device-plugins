package bridge

import (
	"fmt"
	"os"
	"strings"

	"kubevirt.io/kubernetes-device-plugins/pkg/dpm"
)

const (
	BridgesListEnvironmentVariable = "BRIDGES"
	nicsPoolSize                   = 100
)

type BridgeLister struct{}

func (bl BridgeLister) Discover() *dpm.DeviceMap {
	bridgesListRaw := os.Getenv(BridgesListEnvironmentVariable)
	bridges := strings.Split(bridgesListRaw, ",")

	var devices = make(dpm.DeviceMap)

	for _, bridgeName := range bridges {
		for i := 0; i < nicsPoolSize; i++ {
			devices[bridgeName] = append(devices[bridgeName], fmt.Sprintf("%s-nic%d", bridgeName, i))
		}
	}

	return &devices
}

func (bl BridgeLister) NewDevicePlugin(bridge string, nics []string) dpm.DevicePluginInterface {
	return dpm.DevicePluginInterface(newDevicePlugin(bridge, nics))
}
