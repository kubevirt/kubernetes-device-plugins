import time

from kubernetes import client
from kubernetes import watch
from kubernetes.client import rest
from kubernetes.stream import stream


def assert_resource_ready(resource, node, timeout=0):
    _assert_resource_status(
        node=node,
        status_check=lambda resources: resources.get(resource, '0') != '0',
        timeout=timeout
    )


def assert_resource_empty(resource, node, timeout=0):
    _assert_resource_status(
        node=node,
        status_check=lambda resources: resources.get(resource, '0') == '0',
        timeout=timeout
    )


def _assert_resource_status(node, status_check, timeout=0):
    v1 = client.CoreV1Api()

    w = watch.Watch()
    for event in w.stream(v1.list_node, timeout_seconds=timeout):
        if event['object'].metadata.name == node:
            if status_check(event['object'].status.allocatable):
                return

    raise AssertionError('Given resource have not turned into expected state')


def assert_daemon_set_ready(daemon_set, namespace, timeout=0):
    apps_v1 = client.AppsV1Api()

    w = watch.Watch()
    for event in w.stream(apps_v1.list_namespaced_daemon_set, namespace,
                          timeout_seconds=timeout):
        if (event['object'].metadata.name == daemon_set and
                not event['object'].status.number_unavailable and
                event['object'].status.number_available):
            return

    raise AssertionError(
        'Daemon set was not deployed within the given timeout')


def assert_pod_state(pod, namespace, state, timeout=0):
    v1 = client.CoreV1Api()

    w = watch.Watch()
    for event in w.stream(v1.list_namespaced_pod, namespace,
                          timeout_seconds=timeout):
        if (event['object'].metadata.name and
                event['object'].status.phase == state):
            return

    raise AssertionError('Pods have not turned into the expected state')


def run(pod, namespace, command):
    v1 = client.CoreV1Api()

    resp = stream(v1.connect_get_namespaced_pod_exec, pod, namespace,
                  command=['/bin/sh', '-c', command],
                  stderr=True, stdin=False,
                  stdout=True, tty=False)

    return resp


def assert_in_logs(pod, namespace, expected_string, timeout=0):
    v1 = client.CoreV1Api()

    for i in range(0, timeout+1):
        logs = v1.read_namespaced_pod_log(
            pod, namespace, _preload_content=False)
        try:
            assert expected_string in logs.read()
        except AssertionError:
            if i != timeout:
                time.sleep(1)
            else:
                raise
        else:
            return


def assert_pod_exec_ready(pod, namespace, timeout=0):
    for i in range(0, timeout+1):
        try:
            run(pod, namespace, 'echo echo')
        except rest.ApiException:
            if i != timeout:
                time.sleep(1)
            else:
                raise AssertionError(
                    'It is not possible to use exec on the pod')
        else:
            return


def list_daemon_set_pods(daemon_set, namespace):
    v1 = client.CoreV1Api()

    pods = []

    ret = v1.list_namespaced_pod(namespace, watch=False)
    for pod in ret.items:
        for owner in pod.metadata.owner_references:
            if owner.name == daemon_set:
                pods.append(pod.metadata.name)
                break

    return pods
