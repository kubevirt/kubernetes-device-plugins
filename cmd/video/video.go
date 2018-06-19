package main

import (
	"flag"

	"github.com/kubevirt/device-plugin-manager/pkg/dpm"
	"kubevirt.io/kubernetes-device-plugins/pkg/video"
)

func main() {
	flag.Parse()

	manager := dpm.NewManager(video.Lister{})
	manager.Run()
}
