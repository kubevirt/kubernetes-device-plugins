package main

import (
	"flag"
	"os"

	"github.com/golang/glog"
	"kubevirt.io/kubernetes-device-plugins/pkg/dpm"
	"kubevirt.io/kubernetes-device-plugins/pkg/network/bridge"
)

func main() {
	flag.Parse()

	_, bridgesListDefined := os.LookupEnv(bridge.BridgesListEnvironmentVariable)
	if !bridgesListDefined {
		glog.Exit("BRIDGES environment variable must be set in format BRIDGE[,BRIDGE[...]]")
	}

	manager := dpm.NewManager(bridge.BridgeLister{})
	manager.Run()
}
