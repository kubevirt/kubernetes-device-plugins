package main

import (
	"flag"

	"github.com/mpolednik/linux-vfio-k8s-dpi/pkg/dpm"
	"github.com/mpolednik/linux-vfio-k8s-dpi/pkg/kvm"
)

func main() {
	flag.Parse()

	manager := dpm.NewDevicePluginManager(kvm.KVMLister{})
	manager.Run()
}
