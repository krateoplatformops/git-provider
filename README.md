# Git Provider

This is a [Krateo](https://krateo.io) Provider that clones git repositories (eventually applying templates).

## Summary

- [Summary](#summary)
- [Overview](#overview)
- [Examples](#examples)
- [Configuration](#configuration)
  

## Overview

Git Provider clones git repositories and may apply [Mustache templates](https://mustache.github.io). It then pushes the cloned and modified repository to a different location. It provides automatic reconciliation when changes are retrieved from the original repository.

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
| Parameters  | Type | Default | Description |
|---|---|---|---|
| `spec.enableUpdate`  | boolean | `false` | If `true`, the provider performs updates on the repository specified in `toRepo` when newer commits are retrieved from `fromRepo`. |
| `spec.unsupportedCapabilities` | boolean | `false` | If `true` [capabilities not supported by any client implementation](https://github.com/go-git/go-git/blob/4fd9979d5c2940e72bdd6946fec21e02d959f0f6/plumbing/transport/common.go#L310) will not be used by the provider. |
| `spec.[to]\[from]Repo.authMethod` | string | `nil`| Possible values are: `generic`, `bearer`, `gitcookies`. `generic` requires  `secretRef` and `usernameRef`; `generic` requires only `secretRef`; `cookiefile` requires only `secretRef` |
| `spec.[to]\[from]Repo.secretRef` | object | `nil` | Reference to a K8s secret |
| `spec.[to]\[from]Repo.usernameRef` | object | `nil` | Reference to K8s secret |
| `spec.fromRepo.branch` | string | `nil` | Represents the branch to clone from. |
| `spec.fromRepo.path` | string | `/` | Represents the folder to clone from. If not set the entire repository is cloned. |
| `spec.toRepo.branch` | string | `nil` | Represents the branch to populate. If the branch does not exist on remote is created by the provider |
| `spec.toRepo.cloneFromBranch` | string | `nil` | If set, the provider clones the toRepo repository from `cloneFromBranch`, copies the content specified in `fromRepo` into the cloned branch, and pushes the changes to `spec.toRepo.branch`. |



