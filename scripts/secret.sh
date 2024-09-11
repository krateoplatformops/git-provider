#!/bin/bash

set -o allexport
source .env
set +o allexport

kubectl create secret generic git-secret \
    --namespace=default \
    --from-literal=token=${TOKEN} || true

kubectl create secret generic git-username \
    --namespace=default \
    --from-literal=username=${USERNAME} || true

