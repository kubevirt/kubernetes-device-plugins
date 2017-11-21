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

func GetIOMMUGroup(deviceAddress string) (int, error) {
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

func UnbindIOMMUGroup(iommuGroup int) error {
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

func BindIOMMUGroup(iommuGroup int, driver string) error {
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

func ConstructVFIOPath(iommuGroup int) string {
	return filepath.Join("/dev/vfio", strconv.FormatInt(int64(iommuGroup), 10))
}

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

func formatDeviceID(vendorID string, deviceID string) string {
	return strings.Join([]string{vendorID, deviceID}, "_")
}
