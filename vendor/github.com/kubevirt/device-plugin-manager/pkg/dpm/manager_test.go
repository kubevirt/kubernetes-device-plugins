package dpm_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"golang.org/x/net/context"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
	"kubevirt.io/device-plugin-manager/pkg/dpm"
)

// define a fake device plugin
type FakeDevicePlugin struct {
	Name string
}

var callsToStart int

func (dp *FakeDevicePlugin) Start() error {
	callsToStart++
	return nil
}

func (dp *FakeDevicePlugin) ListAndWatch(e *pluginapi.Empty, s pluginapi.DevicePlugin_ListAndWatchServer) error {
	return nil
}

func (dp *FakeDevicePlugin) Allocate(ctx context.Context, r *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	return nil, nil
}

func (FakeDevicePlugin) GetDevicePluginOptions(context.Context, *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	return nil, nil
}

func (FakeDevicePlugin) PreStartContainer(context.Context, *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	return nil, nil
}

// define a fake lister
type FakeLister struct {
	Plugins []string
}

func (l FakeLister) GetResourceNamespace() string {
	return "fake-lister"
}

func (l FakeLister) Discover(pluginListCh chan dpm.PluginNameList) {
	var plugins = make(dpm.PluginNameList, 0)

	for _, name := range l.Plugins {
		plugins = append(plugins, name)
	}

	pluginListCh <- plugins
}

func (l FakeLister) NewPlugin(name string) dpm.PluginInterface {
	return &FakeDevicePlugin{
		Name: name,
	}
}

// tests for the Device Pluging Manager (DPM)
var _ = Describe("DPM", func() {

	var manager *dpm.Manager
	var plugins []string

	BeforeEach(func() {
		callsToStart = 0
		plugins = append(plugins, "hello")
		plugins = append(plugins, "world")
		lister := FakeLister{Plugins: plugins}
		manager = dpm.NewManager(lister)
	})
	Describe("When DPM start", func() {
		Context("With lister containing multiple plugins", func() {
			It("Start API should be called on each plugin", func() {
				go manager.Run()
				Eventually(func() int { return callsToStart }).Should(Equal(len(plugins)))
			})
		})
	})
})
