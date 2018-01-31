# KVM Device Plugin

## Introduction

This software is a kubernetes [device plugin](https://kubernetes.io/docs/concepts/cluster-administration/device-plugins/) that exposes /dev/kvm from the system.

## Building

NOTE: This process is not finalized yet.

### Plain binary

```bash
cd cmd/kvm
go build .
```

### Docker

```bash
docker build -t kvm:latest .
```

## Running

NOTE: This process is not finalized yet.

### Locally
```bash
cd cmd/kvm
sudo ./kvm -v 3 -logtostderr
```

### Docker
```
docker run -it -v /dev:/dev -v /sys:/sys -v /lib/modules:/lib/modules -v /var/lib/kubelet/device-plugins:/var/lib/kubelet/device-plugins --privileged --cap-add=ALL kvm:latest /bin/bash
(in docker image) ./kvm -v 3 -logtostderr
```

## As a DaemonSet

NOTE: This process is not finalized yet.

## API

Node description:

```yaml
Capacity:
 ...
 mpolednik.github.io/kvm:  3
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
              mpolednik.github.io/kvm: 1
      limits:
              mpolednik.github.io/kvm: 1
```

## Issues

### KVM Quantity

Currently, Kubernetes device plugins can only expose quantifiable resources.
KVM itself is "infinite" resource, so it cannot be properly expressed.

This fact is worked around by the plugin: it adds one more KVM resource each
time VM is started. Because we have no way of detecting stopped VMs, the KVM
counter never goes down -- that causes the plugin to consume excessive memory.
