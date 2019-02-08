// License MIT
package main

import (
	"flag"

	"github.com/kubevirt/device-plugin-manager/pkg/dpm"
	"github.com/kubevirt/kubernetes-device-plugins/pkg/fuse"
)

func main() {
	flag.Parse()

	manager := dpm.NewManager(fuse.FuseLister{})
	manager.Run()
}
