#!/bin/bash

export KUBECONFIG="$(find $HOME/Downloads/ -name 'cluster*.kubeconfig' | sort | tail -n1)"
export IO_ENABLE_INSTRUMENTATION='true'

make

./main start --config=./config/local.yaml --kubeconfig=$KUBECONFIG
