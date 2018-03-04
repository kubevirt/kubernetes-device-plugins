package main

import (
	"flag"

	"kubevirt.io/kubernetes-device-plugins/pkg/dpm"
	"kubevirt.io/kubernetes-device-plugins/pkg/kvm"
)

func main() {
	flag.Parse()

	manager := dpm.NewManager(kvm.KVMLister{})
	manager.Run()
}
