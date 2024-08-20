# Git Provider

This is a [Krateo](https://krateo.io) Provider that clones git repositories (eventually applying templates).

## Summary

- [Summary](#summary)
- [Overview](#overview)
- [Examples](#examples)
- [Configuration](#configuration)
  

## Overview

Git Provider clones git repositories and may apply [Mustache templates](https://mustache.github.io). It then pushes the cloned and modified repository to a different location. The templating values are retrieved in a configmap referenced in the custom resource. 
It provides automatic reconciliation when changes are retrieved from the original repository.

Git Provider leverages Krateo [provider-runtime](https://docs.krateo.io/key-concepts/kco/#provider-runtime) a production-grade version of the controller-runtime. 

## Examples

### Provider Installation

```bash
$ helm repo add krateo https://charts.krateo.io
$ helm repo update krateo
$ helm install git-provider krateo/git-provider
```

### Manifest application

```yaml
apiVersion: git.krateo.io/v1alpha1
kind: Repo
metadata:
  name: git-azuredevops-branch-5
spec:
  enableUpdate: false 
  configMapKeyRef:
    key: values
    name: filename-replace-values
    namespace: default
  deletionPolicy: Delete
  fromRepo:
    authMethod: generic 
    branch: main
    path: skeleton/
    usernameRef:
      key: username
      name: github-user
      namespace: default
    secretRef:
      key: token
      name: github-token
      namespace: default
    url: https://github.com/matteogastaldello/fromRepo
  toRepo:
    authMethod: generic
    branch: test-5
    usernameRef:
      key: username
      name: azure-user
      namespace: default
    secretRef:
      key: token
      name: azure-token
      namespace: default
    url: https://matteogastaldello-org@dev.azure.com/matteogastaldello-org/teamproject/_git/repo-generated
  unsupportedCapabilities: true
```

## Configuration
To view the CR configuration visit [this link](https://doc.crds.dev/github.com/krateoplatformops/git-provider).