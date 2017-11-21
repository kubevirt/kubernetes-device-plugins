package pci

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/golang/glog"
)

type DeviceMap map[string][]string

// Discovery discovers all PCI devices within the system.
func Discover() *DeviceMap {
	var devices = make(DeviceMap)

	filepath.Walk("/sys/bus/pci/devices", func(path string, info os.FileInfo, err error) error {
		glog.V(3).Infof("Discovering device in %s", path)
		if info.IsDir() {
			glog.V(3).Infof("Not a device, continuing")
			return nil
		}

		vendorID, deviceID, err := getDeviceVendor(info.Name())
		if err != nil {
			glog.V(3).Infof("Could not process device %s", info.Name())
			return filepath.SkipDir
		}

		deviceClass := formatDeviceID(vendorID, deviceID)
		devices[deviceClass] = append(devices[deviceClass], info.Name())

		return nil
	})

	glog.V(3).Infof("Discovered devices: %s", devices)
	return &devices
}

// getIOMMUGroup finds device's IOMMU group.
func getIOMMUGroup(deviceAddress string) (int, error) {
	iommuPath, err := os.Readlink(filepath.Join("/sys/bus/pci/devices", deviceAddress, "iommu_group"))
	if err != nil {
		glog.Errorf("Could not read IOMMU group for device %s: %s", deviceAddress, err)
		return -1, err
	}

	_, file := filepath.Split(iommuPath)

	iommuGroup, err := strconv.ParseInt(file, 10, 32)
	if err != nil {
		glog.Errorf("Could not parse IOMMU group for device %s: %s", deviceAddress, err)
		return -1, err
	}

	return int(iommuGroup), nil
}

// unbindIOMMUGroup unbinds all devices within the IOMMU group from their drivers.
func unbindIOMMUGroup(iommuGroup int) error {
	glog.V(3).Info("Unbinding all devices in IOMMU group %d", iommuGroup)

	devices, err := ioutil.ReadDir(filepath.Join("/sys/kernel/iommu_groups", strconv.FormatInt(int64(iommuGroup), 10), "devices"))

	for _, dev := range devices {
		glog.V(3).Infof("Unbinding device %s", dev.Name())
		err = ioutil.WriteFile(filepath.Join("/sys/bus/pci/devices", dev.Name(), "driver/unbind"), []byte(dev.Name()), 0400)
		if err != nil {
			glog.V(3).Infof("Device %s not bound to any driver: %s", dev.Name(), err)
		}
	}

	return nil
}

// bindIOMMUGroup binds all devices within the IOMMU group to vfio-pci driver.
func bindIOMMUGroup(iommuGroup int, driver string) error {
	glog.V(3).Info("Binding all devices in IOMMU group %d", iommuGroup)

	devices, err := ioutil.ReadDir(filepath.Join("/sys/kernel/iommu_groups", strconv.FormatInt(int64(iommuGroup), 10), "devices"))

	for _, dev := range devices {
		glog.V(3).Infof("Binding device %s to driver %s", dev.Name(), driver)
		err = ioutil.WriteFile(filepath.Join("/sys/bus/pci/drivers", driver, "bind"), []byte(dev.Name()), 0400)
		if err != nil {
			glog.Errorf("Could not bind %s: %s", dev.Name(), err)
			return err
		}
	}

	return nil
}

// constructVFIOPath is a helper to create dev paths to VFIO endpoints (/dev/vfio/{0-9}+).
func constructVFIOPath(iommuGroup int) string {
	return filepath.Join("/dev/vfio", strconv.FormatInt(int64(iommuGroup), 10))
}

// getDeviceVendor fetches the vendor:device tuple from sysfs. This tupple is used as "device class" within this plugin.
func getDeviceVendor(deviceAddress string) (string, string, error) {
	data, err := ioutil.ReadFile(filepath.Join("/sys/bus/pci/devices", deviceAddress, "vendor"))
	if err != nil {
		glog.Errorf("Could not read vendorID for device %s: %s", deviceAddress, err)
		return "", "", err
	}
	vendorID := strings.Trim(string(data[2:]), "\n")

	data, err = ioutil.ReadFile(filepath.Join("/sys/bus/pci/devices", deviceAddress, "device"))
	if err != nil {
		glog.Errorf("Could not read deviceID for device %s: %s", deviceAddress, err)
		return "", "", err
	}
	deviceID := strings.Trim(string(data[2:]), "\n")

	return vendorID, deviceID, nil
}

// formatDeviceID formats the device class so that the pods can request it in the limits.
// Typically, vendor:device would be used, but the resource name may not contain ":".
func formatDeviceID(vendorID string, deviceID string) string {
	return strings.Join([]string{vendorID, deviceID}, "_")
}
