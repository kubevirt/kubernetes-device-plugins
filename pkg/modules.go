package pci

import (
	"github.com/golang/glog"
	"io/ioutil"
	"os/exec"
)

func loadedModules() []string {
	moduleDirs, err := ioutil.ReadDir("/sys/module")
	if err != nil {
		glog.Errorf("Could not fetch loaded kernel modules: %s", err)
		return []string{}
	}

	var modules []string

	for _, moduleDir := range moduleDirs {
		module := moduleDir.Name()
		modules = append(modules, module)
	}

	return modules
}

func IsModuleLoaded(searchedModule string) bool {
	modules := loadedModules()

	for _, module := range modules {
		if module == searchedModule {
			return true
		}
	}

	return false
}

func LoadModule(module string) error {
	cmd := exec.Command("modprobe", module)

	err := cmd.Run()
	if err != nil {
		glog.Errorf("Modprobe did not succeed: %s", err)
		return err
	}

	return nil
}

func UnloadModule(module string) error {
	cmd := exec.Command("rmmod", module)

	err := cmd.Run()
	if err != nil {
		glog.Errorf("rmmod did not succeed: %s", err)
		return err
	}

	return nil
}
