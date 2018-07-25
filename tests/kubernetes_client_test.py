from kubernetes import client


def test_list_cluster_nodes_using_python_client():
    v1 = client.CoreV1Api()
    node_list = v1.list_node()
    node_name_list = [node.metadata.name for node in node_list.items]
    assert set(['node01', 'node02']) == set(node_name_list)
