# FUSE Device Plugin

## Introduction

This software is a kubernetes [device plugin](https://kubernetes.io/docs/concepts/cluster-administration/device-plugins/) that exposes /dev/fuse from the system.

## Building

NOTE: This process is not finalized yet.

### Plain binary

```bash
cd cmd/fuse
go build .
```

### Docker

```bash
docker build -t fuse:latest .
```

## Running

NOTE: This process is not finalized yet.

### Locally
```bash
cd cmd/fuse
sudo ./fuse -v 3 -logtostderr
```

### Docker
```
docker run -it -v /var/lib/kubelet/device-plugins:/var/lib/kubelet/device-plugins --privileged --cap-add=ALL fuse:latest /bin/bash
(in docker image) ./fuse -v 3 -logtostderr
```

## As a DaemonSet

```
# Deploy the device plugin
kubectl apply -f manifests/fuse-ds.yml
```

## API

Node description:

```yaml
Capacity:
 ...
 devices.kubevirt.io/fuse: "3"
 ...
```

Pod:

```yaml
spec:
  containers:
  - name: demo
    ...
    resources:
      requests:
              devices.kubevirt.io/fuse: "1"
      limits:
              devices.kubevirt.io/fuse: "1"
```

The device does not need a priviliged container to use it, but it does require the `mount` system call and CAP_SETUID. The `mount` system call can be made
available by a seccomp profile.

## Issues

### FUSE Quantity

Currently, Kubernetes device plugins can only expose quantifiable resources.
FUSE itself is "infinite" resource, so it cannot be properly expressed.

This fact is worked around by the plugin: it adds one more FUSE resource each
time VM is started. Because we have no way of detecting stopped VMs, the FUSE
counter never goes down -- that causes the plugin to consume excessive memory.
