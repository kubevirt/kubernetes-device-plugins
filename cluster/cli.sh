#!/bin/bash -e

node=$1

source ./cluster/gocli.sh

$gocli_interactive ssh $node
