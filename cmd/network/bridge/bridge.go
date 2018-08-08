package main

import (
	"flag"

	"github.com/kubevirt/device-plugin-manager/pkg/dpm"
	"github.com/kubevirt/kubernetes-device-plugins/pkg/network/bridge"
)

func main() {
	flag.Parse()

	// _, bridgesListDefined := os.LookupEnv(bridge.BridgesListEnvironmentVariable)
	// if !bridgesListDefined {
	// 	glog.Exit("BRIDGES environment variable must be set in format BRIDGE[,BRIDGE[...]]")
	// }

	manager := dpm.NewManager(bridge.BridgeLister{})
	manager.Run()
}
