package bridge

import (
	"os"
	"strings"

	"github.com/golang/glog"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1alpha"
	"kubevirt.io/kubernetes-device-plugins/pkg/dpm"
)

const (
	BridgesListEnvironmentVariable = "BRIDGES"
	// maximal interface name length (15) - nic index suffix (3)
	maxBridgeNameLength = 12
)

type BridgeLister struct{}

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

func (bl BridgeLister) NewDevicePlugin(bridge string) dpm.DevicePluginInterface {
	glog.V(3).Infof("Creating device plugin %s", bridge)
	ret := &NetworkBridgeDevicePlugin{
		dpm.DevicePlugin{
			Socket:       pluginapi.DevicePluginPath + bridge,
			ResourceName: resourceNamespace + bridge,
		},
		bridge,
		make(chan *Assignment),
	}
	ret.DevicePlugin.Deps = ret

	return ret
}
