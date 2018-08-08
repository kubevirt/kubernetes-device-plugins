package main

import (
	"flag"

	"github.com/kubevirt/device-plugin-manager/pkg/dpm"
	"github.com/kubevirt/kubernetes-device-plugins/pkg/network/bridge"
)

func main() {
	flag.Parse()

	manager := dpm.NewManager(bridge.BridgeLister{})
	manager.Run()
}
