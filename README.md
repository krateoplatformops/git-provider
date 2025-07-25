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

### Manifest Application

As a first step, you need to create a [`kind: Repo` Manifest](#repo-manifest) as shown below and a [ConfigMap](#configmap-manifest) which will contain the templating values.

### File Templating
`git-provider` uses the Mustache library ([see custom delimiter reference](https://github.com/janl/mustache.js/?tab=readme-ov-file#setting-in-templates)) to apply templating. Therefore, you need to specify the custom delimiter you want to use in the first line of the file you want to template. You can see an example [here](https://github.com/krateoplatformops/krateo-v2-template-fireworksapp/blob/5dee9fe1d2de3785eb7e6374ad50e3f8e7b12907/skeleton/chart/values.yaml#L1C1-L1C14).

### File Name Templating
If you need to template the filename of a file, you can only use the delimiters `{{ }}` (e.g., `{{ your-prop }}.yaml`).

#### Repo Manifest
```yaml
apiVersion: git.krateo.io/v1alpha1
kind: Repo
metadata:
  name: test-repo
spec:
  enableUpdate: false
  configMapKeyRef:
    key: values
    name: filename-replace-values
    namespace: default
  fromRepo:
    authMethod: generic
    branch: main
    path: skeleton
    usernameRef:
      key: username
      name: git-username
      namespace: default
    secretRef:
      key: token
      name: git-secret
      namespace: default
    url: https://github.com/your-organization/fromRepo
  toRepo:
    authMethod: generic
    branch: main
    cloneFromBranch: main
    path: /
    secretRef:
      key: token
      name: git-secret
      namespace: default
    usernameRef:
      key: username
      name: git-username
      namespace: default
    url: https://github.com/your-organization/toRepo
  unsupportedCapabilities: true
```

#### Configmap Manifest
```yaml 
apiVersion: v1
kind: ConfigMap
metadata:
  name: filename-replace-values
data:
  values: |
    { 
      "organizationName": "krateo",
      "repositoryName": "testfilename",
      "serviceType": "type",
      "servicePort": "8080",
      "testTemplate": "tplKrateo"
    }
```


## Environment Variables

| Environment Variable | Type | Default Value | Description |
|---------------------|------|---------------|-------------|
| `GIT_PROVIDER_DEBUG` | bool | `false` | Run with debug logging |
| `GIT_PROVIDER_SYNC_PERIOD` | duration | `1h` | Controller manager sync period (e.g., 300ms, 1.5h, or 2h45m) |
| `GIT_PROVIDER_POLL_INTERVAL` | duration | `2m` | Poll interval controls how often an individual resource should be checked for drift |
| `GIT_PROVIDER_MAX_RECONCILE_RATE` | int | `5` | The number of concurrent reconciles for each controller. Maximum number of resources that can be reconciled at the same time |
| `GIT_PROVIDER_LEADER_ELECTION` | bool | `false` | Use leader election for the controller manager |
| `GIT_PROVIDER_MAX_ERROR_RETRY_INTERVAL` | duration | `1m` | The maximum interval between retries when an error occurs. Should be less than half of the poll interval |
| `GIT_PROVIDER_MIN_ERROR_RETRY_INTERVAL` | duration | `1s` | The minimum interval between retries when an error occurs. Should be less than max-error-retry-interval |

## Configuration
To view the CR configuration visit [this link](https://doc.crds.dev/github.com/krateoplatformops/git-provider).