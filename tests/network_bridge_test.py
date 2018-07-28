from contextlib import contextmanager
import itertools
import yaml

from kubernetes import client

from . import kubeutils
from . import netutils
from . import nodes


def test_bridge_device_plugin(namespace):
    with open('./tests/network_bridge_test_cm.yml') as f:
        config_map = yaml.load(f)

    with open('./tests/network_bridge_test_ds.yml') as f:
        device_plugin_daemon_set = yaml.load(f)

    with open('./tests/network_bridge_test_pods.yml') as f:
        consumer_pods = yaml.load(f)

    v1 = client.CoreV1Api()
    apps_v1 = client.AppsV1Api()

    # Create a config map listing names of bridges that will be exposed as
    # available resources
    v1.create_namespaced_config_map(
        body=config_map,
        namespace=namespace
    )

    # Deploy bridge device plugin as a daemon set
    apps_v1.create_namespaced_daemon_set(
        body=device_plugin_daemon_set,
        namespace=namespace
    )

    # Wait until the daemon set is ready on all nodes
    kubeutils.assert_daemon_set_ready(
        daemon_set=device_plugin_daemon_set['metadata']['name'],
        namespace=namespace,
        timeout=300
    )

    # Verify that all device plugin's instances started successfully
    for pod in kubeutils.list_daemon_set_pods(
        daemon_set=device_plugin_daemon_set['metadata']['name'],
        namespace=namespace
    ):
        kubeutils.assert_in_logs(
            pod,
            namespace=namespace,
            expected_string='Serving requests',
            timeout=5
        )

    # Verify that resources are reported, but are not allocatable
    for node, resource in itertools.product(
        ['node01', 'node02'],
        ['bridge.network.kubevirt.io/mybr1',
         'bridge.network.kubevirt.io/mybr2']
    ):
        kubeutils.assert_resource_empty(
            resource,
            node=node,
            timeout=60
        )

    # Deploy two pods on each node, all of them requesting both available
    # bridge resources
    for pod in consumer_pods['items']:
        v1.create_namespaced_pod(
            body=pod,
            namespace=namespace
        )

    # Wait for consumer pods to become pending since requested resource is not
    # available on any node
    for pod in consumer_pods['items']:
        kubeutils.assert_pod_state(
            pod['metadata']['name'],
            namespace=namespace,
            state='Pending',
            timeout=300
        )

    # Configure networks across cluster nodes
    with _configure_host_networking('node01'), \
            _configure_host_networking('node02'):

        # Check that resources are finally reported as allocatable
        for node, resource in itertools.product(
            ['node01', 'node02'],
            ['bridge.network.kubevirt.io/mybr1',
             'bridge.network.kubevirt.io/mybr2']
        ):
            kubeutils.assert_resource_ready(
                resource,
                node=node,
                timeout=60
            )

        # Check that pods were scheduled and successfully started
        for pod in consumer_pods['items']:
            kubeutils.assert_pod_state(
                pod['metadata']['name'],
                namespace=namespace,
                state='Running',
                timeout=300
            )
            kubeutils.assert_pod_exec_ready(
                pod['metadata']['name'],
                namespace=namespace,
                timeout=60
            )

        # Configure addresses on pod interfaces connected to bridge resource
        _set_address_on_resource_interface(
            pod='bridge-consumer-1',
            namespace=namespace,
            resource='bridge.network.kubevirt.io/mybr1',
            address='192.168.1.21/24'
        )
        _set_address_on_resource_interface(
            pod='bridge-consumer-4',
            namespace=namespace,
            resource='bridge.network.kubevirt.io/mybr1',
            address='192.168.1.24/24'
        )

        # Check if there is connectivity between two pods running on different
        # nodes
        interface = _get_interface_of_resource(
            pod='bridge-consumer-1',
            namespace=namespace,
            resource='bridge.network.kubevirt.io/mybr1'
        )
        netutils.assert_ping(
            pod='bridge-consumer-1',
            namespace=namespace,
            destination='192.168.1.24',
            interface=interface
        )


@contextmanager
def _configure_host_networking(node):
    # On KubeVirt CI nodes, forwarding is disabled by default, enable it
    nodes.ssh(node, ['sudo', 'iptables', '--policy', 'FORWARD', 'ACCEPT'])

    # Create two VLAN interfaces on top of eth0 and connect them to separate
    # bridges. That will give us two isolated networks between nodes
    for net_id in [1, 2]:
        nodes.ssh(node, """
sudo ip link add link eth0 name eth0.{net_id} type vlan id {net_id} &&
sudo ip link add name mybr{net_id} type bridge &&
sudo ip link set eth0.{net_id} master mybr{net_id} &&
sudo ip link set eth0.{net_id} up &&
sudo ip link set mybr{net_id} up
        """.format(net_id=net_id).split())
    try:
        yield
    finally:
        for net_id in [1, 2]:
            nodes.ssh(node, """
sudo ip link delete eth0.{net_id} &&
sudo ip link delete mybr{net_id}
            """.format(net_id=net_id).split())


def _set_address_on_resource_interface(pod, namespace, resource, address):
    interface = _get_interface_of_resource(pod, namespace, resource)
    kubeutils.run(
        pod=pod,
        namespace=namespace,
        command='ip address add {} dev {}'.format(address, interface)
    )


def _get_interface_of_resource(pod, namespace, resource):
    interfaces_by_resource = netutils.read_interfaces_by_resource(
        pod=pod,
        namespace=namespace
    )
    return interfaces_by_resource[resource][0]['name']
