package dpm

import (
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1alpha"
)

type DevicePlugin interface {
	pluginapi.DevicePluginServer
	Run() error
	Stop() error
}
