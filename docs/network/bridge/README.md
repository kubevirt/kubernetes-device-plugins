# Network Bridge Device Plugin

This device plugin allows user to attach pods to bridges running on nodes.

## Usage

User must specify list of bridges that should be exposed to pods. Please note,
that bridges must be manually precreated on all nodes.

```
kubectl create configmap devicepluginnetworkbridge --from-literal=bridges="br0,br1"
```

Deploy the device plugin using daemon set.

```
kubectl create -f cmd/network/bridge/daemonset.yml
```

Define a pod requesting secondary interface connected to bridge `br0`.

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: demo-pod
spec:
  containers:
  - name: nginx
    image: nginx:latest
    resources:
      limits:
        bridge.network.kubevirt.io/br0: 1
```

```
kubectl create -f pod-example.yml
```

Once the pod is created is should contain a secondary interface connected to
the bridge.
