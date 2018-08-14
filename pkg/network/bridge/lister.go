package bridge

import (
	"strings"

	"github.com/golang/glog"
	"github.com/kubevirt/device-plugin-manager/pkg/dpm"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

const (
	resourceNamespace              = "bridge.network.kubevirt.io"
	BridgesListEnvironmentVariable = "BRIDGES"
	// maximal interface name length (15) - nic index suffix (3)
	maxBridgeNameLength = 12
	configMapName       = "device-plugin-network-bridge"
)

func getBridgeList(configMap *v1.ConfigMap) dpm.PluginNameList {
	var plugins = make(dpm.PluginNameList, 0)

	bridges := strings.Split(configMap.Data["bridges"], ",")

	for _, bridgeName := range bridges {
		if len(bridgeName) > maxBridgeNameLength {
			glog.Fatalf("Bridge name (%s) cannot be longer than %d characters", bridgeName, maxBridgeNameLength)
		}
		plugins = append(plugins, bridgeName)
	}

	return plugins
}

type BridgeLister struct{}

func (bl BridgeLister) GetResourceNamespace() string {
	return resourceNamespace
}

func (bl BridgeLister) Discover(pluginListCh chan dpm.PluginNameList) {
	config, err := rest.InClusterConfig()
	if err != nil {
		glog.Fatalf(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		glog.Fatalf(err.Error())

	}

	watchlist := cache.NewListWatchFromClient(clientset.CoreV1().RESTClient(), string(v1.ResourceConfigMaps), "kube-system", fields.Everything())

	_, informer := cache.NewInformer(watchlist, &v1.ConfigMap{}, 0, cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			cm := obj.(*v1.ConfigMap)
			if cm.GetName() == configMapName {
				glog.V(3).Infof("Config map was updated: %v", cm.Data)
				pluginListCh <- getBridgeList(cm)
			}
		},
		DeleteFunc: func(obj interface{}) {
			cm := obj.(*v1.ConfigMap)
			if cm.GetName() == configMapName {
				glog.V(3).Infof("Config map was deleted: %v", cm.Data)
				var plugins = make(dpm.PluginNameList, 0)
				pluginListCh <- plugins
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			cm := newObj.(*v1.ConfigMap)
			if cm.GetName() == configMapName {
				glog.V(3).Infof("Config map was updated: %v", cm.Data)
				pluginListCh <- getBridgeList(cm)
			}
		},
	})

	stop := make(chan struct{})
	defer close(stop)
	informer.Run(stop)
}

func (bl BridgeLister) NewPlugin(bridge string) dpm.PluginInterface {
	glog.V(3).Infof("Creating device plugin %s", bridge)
	return &NetworkBridgeDevicePlugin{
		bridge:       bridge,
		assignmentCh: make(chan *Assignment),
		//cleaner:      newCleaner(),
	}
}
