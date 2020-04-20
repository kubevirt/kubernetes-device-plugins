# This repository was archived

Please read https://github.com/kubevirt/kubernetes-device-plugins/issues/64.

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

* [VFIO](docs/README.vfio.md)
* [KVM](docs/README.kvm.md)
* Network
  * [Bridge](docs/README.bridge.md)

## Creating Device Plugin

See Device Plugin Manager documentaion on
https://godoc.org/github.com/kubevirt/device-plugin-manager/pkg/dpm

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
```

Deploy local Kubernetes cluster:

```
make cluster-up
```

Destroy local Kubernetes cluster:

```
make cluster-down
```

Build all device plugin images and push them to local Kubernetes cluster registry:

```
make cluster-sync
```

Build specific device plugin image and push it to local Kubernetes cluster registry:

```
make cluster-sync-network-bridge
```

Access cluster kubectl:

```
./cluster/kubectl.sh ...
```

Access cluster node via ssh:

```
./cluster/cli.sh node01
```

Run e2e tests (on running cluster):

```
make functests
```
