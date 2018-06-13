package main

import (
	"flag"

	"github.com/kubevirt/device-plugin-manager/pkg/dpm"
	"kubevirt.io/kubernetes-device-plugins/pkg/random"
)

func main() {
	flag.Parse()

	manager := dpm.NewManager(random.Lister{})
	manager.Run()
}
