#!/bin/bash

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

${SCRIPT_DIR}/kind-down.sh
${SCRIPT_DIR}/kind-up.sh

${SCRIPT_DIR}/build.sh

${SCRIPT_DIR}/secret.sh


kubectl apply -f crds/
kubectl apply -f manifests/ns.yaml
kubectl apply -f manifests/sa.yaml
kubectl apply -f manifests/rbac.yaml
kubectl apply -f manifests/deploy.yaml

