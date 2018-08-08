package bridge

import (
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/kubevirt/device-plugin-manager/pkg/dpm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	resourceNamespace              = "bridge.network.kubevirt.io"
	BridgesListEnvironmentVariable = "BRIDGES"
	// maximal interface name length (15) - nic index suffix (3)
	maxBridgeNameLength = 12
)

type BridgeLister struct{}

func (bl BridgeLister) GetResourceNamespace() string {
	return resourceNamespace
}

func (bl BridgeLister) Discover(pluginListCh chan dpm.PluginNameList) {
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	for {
		var plugins = make(dpm.PluginNameList, 0)

		configmap, err := clientset.CoreV1().ConfigMaps("kube-system").Get("device-plugin-network-bridge", metav1.GetOptions{})
		if err != nil {
			panic(err.Error())
		}

		bridges := strings.Split(configmap.Data["bridges"], ",")

		for _, bridgeName := range bridges {
			if len(bridgeName) > maxBridgeNameLength {
				glog.Fatalf("Bridge name (%s) cannot be longer than %d characters", bridgeName, maxBridgeNameLength)
			}
			plugins = append(plugins, bridgeName)
		}

		pluginListCh <- plugins
		time.Sleep(10 * time.Second)
	}
}

func (bl BridgeLister) NewPlugin(bridge string) dpm.PluginInterface {
	glog.V(3).Infof("Creating device plugin %s", bridge)
	return &NetworkBridgeDevicePlugin{
		bridge:       bridge,
		assignmentCh: make(chan *Assignment),
		//cleaner:      newCleaner(),
	}
}
