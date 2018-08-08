package bridge

import (
	"strings"

	"github.com/golang/glog"
	"github.com/kubevirt/device-plugin-manager/pkg/dpm"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
)

func GetBridgeList(configMap *v1.ConfigMap) dpm.PluginNameList {
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
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	configmap, err := clientset.CoreV1().ConfigMaps("kube-system").Get("device-plugin-network-bridge", metav1.GetOptions{})
	if err != nil {
		panic(err.Error())
	}

	pluginListCh <- GetBridgeList(configmap)

	watchlist := cache.NewListWatchFromClient(clientset.CoreV1().RESTClient(), string(v1.ResourceConfigMaps), v1.NamespaceAll, fields.Everything())

	_, informer := cache.NewInformer(watchlist, &v1.ConfigMap{}, 0, cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, newObj interface{}) {
			cm := newObj.(*v1.ConfigMap)
			if cm.GetName() == "device-plugin-network-bridge" {
				glog.Infoln(cm.Data)
				pluginListCh <- GetBridgeList(cm)
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
