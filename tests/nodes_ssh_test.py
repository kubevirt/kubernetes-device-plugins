from . import nodes


def test_connect_to_cluster_node_using_ssh():
    _test_node_hostname('node01')
    _test_node_hostname('node02')


def _test_node_hostname(node):
    stdout = nodes.ssh(node, ['hostname'])
    assert stdout[0] == node
