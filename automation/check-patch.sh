#!/bin/bash -xe

install_deps() {
    pip install pytest
}

main() {
    echo "TODO: Add functional tests"
}

[[ "${BASH_SOURCE[0]}" == "$0" ]] && main "$@"
