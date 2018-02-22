# Network Bridge Device Plugin

This device plugin allows user to attach pods to existing bridges on nodes.

> **Note:** Currently device plugins are feature gated, you need to open it
> in order to test this plugin, for minikube this works as follows:
> `minikube start --feature-gates=DevicePlugins=true --vm-driver kvm2 --network-plugin=cni`

## Usage

The plugin does not create bridges, thus it's up to the user to create any
bridge on the nodes of a cluster.
If a bridge doesn't exist, the user can create it (with root privileges) as
follows:

```bash
$ ip link create mybr0 type bridge
$ ip link set dev mybr0 up
$ ip link show mybr0
6: mybr0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UNKNOWN mode DEFAULT group default qlen 1000
    link/ether fe:1b:e5:82:d3:b5 brd ff:ff:ff:ff:ff:ff
```

Now the user must specify list of bridges that should be exposed to pods:

```bash
$ kubectl create configmap device-plugin-network-bridge --from-literal=bridges="mybr0"
configmap "device-plugin-network-bridge" created
```

**Note:** Multiple bridges can be exposed by using a comma separated list of
names: `mybr0,mybr1,mybr2`

Once this is done, the device plugin can be deployed using daemon set:

```
$ kubectl apply -f https://raw.githubusercontent.com/kubevirt/kubernetes-device-plugins/master/manifests/bridge-ds.yml
daemonset "device-plugin-network-bridge" created

$ kubectl get pods
NAME                                 READY     STATUS    RESTARTS   AGE
device-plugin-network-bridge-745x4   1/1       Running   0          5m
```

If the daemonset is running the user can define a pod requesting secondary
interface connected to bridge `mybr0`.

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: bridge-consumer
spec:
  containers:
  - name: busybox
    image: busybox
    command: ["/bin/sleep", "123"]
    resources:
      limits:
        bridge.network.kubevirt.io/mybr0: 1
```

```bash
$ kubectl apply -f https://raw.githubusercontent.com/kubevirt/kubernetes-device-plugins/master/docs/network-bridge/example-pod.yml
pod "bridge-consumer" created
```

Once the pod is created is should contain a secondary interface connected to
the bridge.

```bash
$ kubectl exec -it bridge-consumer ip link
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue qlen 1
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
2: sit0@NONE: <NOARP> mtu 1480 qdisc noop qlen 1
    link/sit 0.0.0.0 brd 0.0.0.0
4: eth0@if10: <BROADCAST,MULTICAST,UP,LOWER_UP,M-DOWN> mtu 1460 qdisc noqueue 
    link/ether 0a:58:0a:01:00:04 brd ff:ff:ff:ff:ff:ff
11: mybr0-20@if12: <BROADCAST,MULTICAST,M-DOWN> mtu 1500 qdisc noop 
    link/ether 96:21:62:8b:11:9e brd ff:ff:ff:ff:ff:ff
```
