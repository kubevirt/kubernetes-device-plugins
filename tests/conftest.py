import random

from kubernetes import client
from kubernetes import config
import pytest
import urllib3


@pytest.fixture(scope="session", autouse=True)
def disable_insecure_requests_warning():
    urllib3.disable_warnings(urllib3.exceptions.InsecureRequestWarning)


@pytest.fixture(scope="session", autouse=True)
def load_local_cluster_kubeconfig():
    config.load_kube_config('./kubeconfig')


@pytest.fixture
def namespace():
    v1 = client.CoreV1Api()

    namespace = 'test-namespace-{:03}'.format(random.randint(0, 999))

    v1.create_namespace(
        body=client.V1Namespace(
            metadata=client.V1ObjectMeta(name=namespace)
        )
    )

    yield namespace

    v1.delete_namespace(name=namespace, body=client.V1DeleteOptions())
