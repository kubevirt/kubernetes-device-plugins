# Collection of Kubernetes Device Plugins

```
EARLY STAGE PROJECT

(yes, the logo is literally photo of whiteboard drawing)
```

![Logo](/docs/logo.jpg)

This repository aims to group various device plugins by providing unified
handling for common patterns.

## Current Device Plugins

[VFIO](docs/vfio/README.md)

## Building & Deploying

Each plugin may have it's own build instructions.

* [VFIO](docs/vfio/README.md)

## Creating Device Plugin

### Step 1: DeviceLister

The DeviceLister interface is defined in [pkg/dpm/lister.go](pkg/dpm/lister.go).
Implementation of DeviceLister is responsible for initially finding devices to
report, and providing constructor for DevicePlugin of the device.

### Step 2: DevicePlugin

The DevicePluginInterface interface (duh) represents the smallest interface
that is acceptable by the DevicePluginManager. To save a bit of gRPC
boilerplate, [pkg/dpm/plugin.go](pkg/dpm/plugin.go) implements a DevicePlugin
struct that is able to satisfy all non-endpoint methods. Embedding this
interface only leaves you to implement the gRPC call implementation itself.

### Step 3: DevicePluginManager

Given DeviceLister, DevicePluginManager is capable of handling lifetime of the
device plugin, it's devices and gRPC server.
