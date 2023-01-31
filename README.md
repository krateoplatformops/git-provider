# Git Provider

This is a [Krateo](https://krateoplatformops.github.io/) Provider that clones git repositories (eventually applying templates).

## Getting Started

You’ll need a Kubernetes cluster to run against. 

> You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.


### Running on the cluster

1. Install the provider:

```sh
$ helm repo add krateo https://charts.krateo.io
$ helm repo update krateo
$ helm install git-provider krateo/git-provider
```

2. Install Instances of Custom Resources:

```sh
$ kubectl apply -f samples/
```

### Test It Out

1. Start a local cluster using [KIND](https://sigs.k8s.io/kind):

```sh
$ make kind-up
```

2. Run your provider (this will run in the foreground, so switch to a new terminal if you want to leave it running):

```sh
$ make dev
```

### Modifying the API definitions
If you are editing the API definitions, generate the CRDs using:

```sh
$ make generate
```

**NOTE:** Run `make help` for more information on all potential `make` targets
