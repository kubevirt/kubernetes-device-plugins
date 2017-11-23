# linux-vfio-k8s-dpi

## Introduction

This software is a kubernetes [device plugin](https://kubernetes.io/docs/concepts/cluster-administration/device-plugins/) that exposes PCI devices from the underlying system and makes them available as resources for the kubernetes node.

## Building

NOTE: This process is not finalized yet.

```bash
cd cmd
go build server.go
```

## Running

NOTE: This process is not finalized yet.

### Locally
```bash
cd cmd
sudo ./server -v 3 -logtostderr
```

## As a DaemonSet

NOTE: This process is not finalized yet.

## API

Node description:

```yaml
Capacity:
 ...
 mpolednik.github.io/10b5_8747:  3
 mpolednik.github.io/10de_0fbc:  1
 mpolednik.github.io/10de_13ba:  1
 mpolednik.github.io/10de_13f2:  2
 mpolednik.github.io/10de_1bb3:  1
 ...
```

Devices are reported as scalar resources, identified by vendor ID and device ID. The format shown in node description is `namespace/vendorID_product_ID: #ofAvailableDevices)`.

Pod:

```yaml
spec:
  containers:
  - name: demo
    ...
    resources:
      requests:
              mpolednik.github.io/10de_1bb3: 1
      limits:
              mpolednik.github.io/10de_1bb3: 1
```

Pod device assignment is derived from device plugin requirements. Snippet above would make sure that

* the pod is scheduled onto node with Tesla P4 installed,
* the device is bound to vfio-pci,
* pod sees VFIO path (/dev/vfio/*).

## Issues

### IOMMU

Currently, IOMMU groups are not enforced at the plugin's level in reporting capabilities. When allocating a device from group that contains other devices, all devices are unbound from their respective drivers but not assigned or reported as unhealthy. This has an unfortunate consequence, best demonstrated by example:

Let `1` be IOMMU group containing devices `A`, `B` and `C`.

If container `a` requests device `A`,

* `A`, `B` and `C` are unbound from their original driver,
* `A`, `B` and `C` are bound to vfio-pci driver,
* `a` gets it's host/container path of the VFIO endpoint (/dev/vfio/1 in this case).

If then container `b` requests device `B`,

* the device is already bound to vfio-pci (which is not a problem),
* `b` is assigned host/container path to the VFIO endpoint (which, same as with `a`, is /dev/vfio/1).

If we start a VM `α` in container `a` with device `A` assigned, and VM `β` in container `b` with device `B` assigned, only one of these VMs would start successfully: we've violated IOMMU restrictions.

The scenario can be solved in several ways -

* assign the whole group to the container,
* report devices `B`, `C` as unhealthy when `a` is created, or devices `A`, `C` when `b` is created.
* ?

The first approach is not feasible as IOMMU groups could be different on another node, e.g. `1` would contain `A` whereas `B` and `C` might be in different group(s).

The second approach has an issue on it's own: when `a` or `b` is destroyed, the plugin is not informed of such event. Adding that capability to the device plugin API may not be desired as it implies a stateful nature of the device. Depending on other device plugin's state management needs, it could turn out to be the right solution.
