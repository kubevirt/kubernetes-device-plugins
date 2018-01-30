# Network Bridge Device Plugin

POC. WIP.

## Build

```
cd cmd/network/bridge
go build
```

## Run

Device plugin running.

```
BRIDGES="br0,br1" ./bridge -v 3 -logtostderr
```

## Use

Create a pod requesting connection to the bridge.

```
apiVersion: v1
kind: Pod
metadata:
  name: demo-pod3
spec:
  containers:
    - name: nginx
      image: nginx:1.7.9
      resources:
        limits:
          bridge.network.kubevirt.io/br0: 1
```
