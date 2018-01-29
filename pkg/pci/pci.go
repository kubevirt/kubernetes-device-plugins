package pci

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/golang/glog"
	"github.com/mpolednik/linux-vfio-k8s-dpi/pkg/dpm"
)

type PCILister struct{}

// Discovery discovers all PCI devices within the system.
func (pci PCILister) Discover() *dpm.DeviceMap {
	var devices = make(dpm.DeviceMap)

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

func (pci PCILister) NewDevicePlugin(deviceID string, deviceIDs []string) dpm.DevicePlugin {
	return dpm.DevicePlugin(newDevicePlugin(deviceID, deviceIDs))
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

type walkFunc func(deviceAddress string, err error) error

// TODO: not really recursive, find better name?
func walkIOMMUGroupDevices(iommuGroup int, walkFn walkFunc) error {
	devices, err := ioutil.ReadDir(filepath.Join("/sys/kernel/iommu_groups", strconv.FormatInt(int64(iommuGroup), 10), "devices"))
	if err != nil {
		return err
	}

	for _, dev := range devices {
		err = walkFn(dev.Name(), err)
		if err != nil {
			return err
		}
	}

	return err
}

func probeIOMMUGroup(iommuGroup int) error {
	glog.V(3).Infof("Probing all devices in IOMMU group %d", iommuGroup)

	err := walkIOMMUGroupDevices(iommuGroup, func(deviceAddress string, err error) error {
		err = safeWrite(filepath.Join("/sys/bus/pci/drivers_probe"), []byte(deviceAddress), 0400)
		glog.V(3).Infof("Probing device %s", deviceAddress)
		if err != nil {
			glog.Errorf("Could not probe device %d", deviceAddress)
			return err
		}

		return nil
	})
	if err != nil {
		glog.Error("Probing failed")
		return err
	}

	return nil
}

// overrideIOMMUGroup binds all devices within the IOMMU group to vfio-pci driver.
func overrideIOMMUGroup(iommuGroup int, driver string) error {
	glog.V(3).Infof("Overriding all device drivers in IOMMU group %d to driver %s", iommuGroup, driver)

	err := walkIOMMUGroupDevices(iommuGroup, func(deviceAddress string, err error) error {
		glog.V(3).Infof("Overriding device %s driver to %s", deviceAddress, driver)
		err = driverOverride(deviceAddress, driver)
		if err != nil {
			glog.Errorf("Could not override device %s with driver %s: %s", deviceAddress, driver, err)
			return err
		}

		return nil
	})
	if err != nil {
		glog.Error("Overriding failed")
		return err
	}

	return nil
}

// unbindIOMMUGroup unbinds all devices within the IOMMU group from their drivers.
func unbindIOMMUGroup(iommuGroup int) error {
	glog.V(3).Infof("Unbinding all devices in IOMMU group %d", iommuGroup)

	err := walkIOMMUGroupDevices(iommuGroup, func(deviceAddress string, err error) error {
		oldDriver, err := os.Readlink(filepath.Join("/sys/bus/pci/devices", deviceAddress, "driver"))
		if err != nil {
			glog.V(3).Infof("Device %s not bound to any driver: %s", deviceAddress, err)
			// Not important, log message is enough.
			return nil
		}
		_, oldDriver = filepath.Split(oldDriver)
		glog.V(3).Infof("Unbinding device %s (previous driver: %s)", deviceAddress, oldDriver)

		// We don't care about error here - it could be race in unbind.
		safeWrite(filepath.Join("/sys/bus/pci/devices", deviceAddress, "driver/unbind"), []byte(deviceAddress), 0400)

		return nil
	})
	if err != nil {
		glog.Error("Overriding failed")
		return err
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

func driverOverride(deviceAddress string, driver string) error {
	return safeWrite(filepath.Join("/sys/bus/pci/devices", deviceAddress, "driver_override"), []byte(driver), 0400)
}

func probe(deviceAddress string) error {
	return safeWrite(filepath.Join("/sys/bus/pci/drivers_probe"), []byte(deviceAddress), 0400)
}

// safeWrite is like ioutil.WriteFile except without O_CREATE, O_TRUNC but with O_SYNC.
func safeWrite(filename string, data []byte, perm os.FileMode) error {
	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_SYNC, perm)
	if err != nil {
		return err
	}
	n, err := f.Write(data)
	if err == nil && n < len(data) {
		err = io.ErrShortWrite
	}
	if err1 := f.Close(); err == nil {
		err = err1
	}

	return err
}
