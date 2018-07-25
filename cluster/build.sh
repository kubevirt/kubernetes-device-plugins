#!/bin/bash -e

plugin=$1

source ./cluster/gocli.sh

registry_port=$($gocli ports registry | tr -d '\r')
registry=localhost:$registry_port

REGISTRY=$registry make docker-build-$plugin
