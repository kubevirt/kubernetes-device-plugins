from kubernetes import config
import pytest
import urllib3


@pytest.fixture(scope="session", autouse=True)
def disable_insecure_requests_warning():
    urllib3.disable_warnings(urllib3.exceptions.InsecureRequestWarning)


@pytest.fixture(scope="session", autouse=True)
def load_local_cluster_kubeconfig():
    config.load_kube_config('./kubeconfig')
