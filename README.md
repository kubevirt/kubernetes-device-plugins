# Collection of Kubernetes Device Plugins

[![Build Status](https://travis-ci.org/kubevirt/kubernetes-device-plugins.svg?branch=master)](https://travis-ci.org/kubevirt/kubernetes-device-plugins)

```
EARLY STAGE PROJECT

(yes, the logo is literally photo of whiteboard drawing)
```

![Logo](/docs/logo.jpg)

This repository aims to group various device plugins by providing unified
handling for common patterns.

## Current Device Plugins

> **Note:** When testing these device plugins ensure to open the feature gate
> using `kubelet`'s `--feature-gates=DevicePlugins=true`

Each plugin may have it's own build instructions in the linked README.md.

* [VFIO](docs/vfio/README.md)
* [KVM](docs/kvm/README.md)
* Network
  * [Bridge](docs/network/bridge/README.md)

## Creating Device Plugin

See Device Plugin Manager documentaion on
https://godoc.org/github.com/kubevirt/kubernetes-device-plugins/pkg/dpm

## Develop

Build all plugins:

```
make build
```

Build specific plugins:

```
make build-vfio
make build-network-bridge
```

Test all modules:

```
make test
```

Test specific modules:

```
make test-cmd-vfio
make test-pkg-dpm
```
