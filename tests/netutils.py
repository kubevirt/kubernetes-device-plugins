import collections
import json

from . import kubeutils


RESOURCES_VARIABLE_NAME_PREFIX = 'NETWORK_INTERFACE_RESOURCES_'


def read_interfaces_by_resource(pod, namespace):
    interfaces_by_resource = collections.defaultdict(list)

    env_raw = kubeutils.run(pod, namespace, 'env')
    env = dict([pair.split('=', 1)
                for pair in [line for line in env_raw.split('\n')[:-1]]])

    for name, value_raw in env.items():
        if name.startswith(RESOURCES_VARIABLE_NAME_PREFIX):
            value = json.loads(value_raw)
            interfaces_by_resource[value['name']].extend(value['interfaces'])

    return dict(interfaces_by_resource)


def assert_ping(pod, namespace, destination, interface):
    output = kubeutils.run(
        pod, namespace,
        'ping {} -W 5 -c 5 -I {}'.format(destination, interface))
    assert '100% packet loss' not in output
