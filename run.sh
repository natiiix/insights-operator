#!/bin/bash

set -e

export KUBECONFIG="$(find $HOME/Downloads/ -name 'cluster*.kubeconfig' | sort | tail -n1)"
#export KUBECONFIG=$HOME/Downloads/kubeconfig
export IO_ENABLE_INSTRUMENTATION='true'
#export IO_READ_TRIGGER_PERIOD=60
export ENV='dev'

make

bin/insights-operator start --config=./config/local.yaml --kubeconfig=$KUBECONFIG
