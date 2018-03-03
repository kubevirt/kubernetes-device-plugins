package bridge

import (
	"os"
	"strings"

	"github.com/golang/glog"
	"kubevirt.io/kubernetes-device-plugins/pkg/dpm"
)

const (
	resourceNamespace              = "bridge.network.kubevirt.io/"
	BridgesListEnvironmentVariable = "BRIDGES"
	// maximal interface name length (15) - nic index suffix (3)
	maxBridgeNameLength = 12
)

type BridgeLister struct{}

func (bl BridgeLister) GetResourceName() string {
	return resourceNamespace
}

func (bl BridgeLister) Discover(pluginListCh chan dpm.PluginList) {
	var plugins = make(dpm.PluginList, 0)

	bridgesListRaw := os.Getenv(BridgesListEnvironmentVariable)
	bridges := strings.Split(bridgesListRaw, ",")

	for _, bridgeName := range bridges {
		if len(bridgeName) > maxBridgeNameLength {
			glog.Fatalf("Bridge name (%s) cannot be longer than %d characters", bridgeName, maxBridgeNameLength)
		}
		plugins = append(plugins, bridgeName)
	}

	pluginListCh <- plugins
}

func (bl BridgeLister) NewDevicePlugin(bridge string) dpm.DevicePluginImplementationInterface {
	glog.V(3).Infof("Creating device plugin %s", bridge)
	return &NetworkBridgeDevicePlugin{
		bridge:       bridge,
		assignmentCh: make(chan *Assignment),
	}
}
