# ctlptl

[![Build Status](https://circleci.com/gh/tilt-dev/ctlptl/tree/main.svg?style=shield)](https://circleci.com/gh/tilt-dev/ctlptl)
[![GoDoc](https://godoc.org/github.com/tilt-dev/ctlptl?status.svg)](https://pkg.go.dev/github.com/tilt-dev/ctlptl)

Want to mess around with Kubernetes, but don't want to spend an ocean on
hardware?

Maybe you need a `ctlptl`.

## What is ctlptl?

`ctlptl` (pronounced "cattle patrol") is a CLI for declaratively setting up
local Kubernetes clusters.

Inspired by `kubectl` and
[ClusterAPI's](https://github.com/kubernetes-sigs/cluster-api) `clusterctl`, you
declare your local cluster with YAML and use `ctlptl` to set it up.

## How do I install it?

Install your cluster of choice: [Docker for
Desktop](https://www.docker.com/products/docker-desktop),
[Kind](https://kind.sigs.k8s.io/), or
[Minikube](https://minikube.sigs.k8s.io/). Then run:

### Homebrew (Mac/Linux)

```
brew install tilt-dev/tap/ctlptl
```

### Scoop (Windows)

```
scoop bucket add tilt-dev https://github.com/tilt-dev/scoop-bucket
scoop install ctlptl
```

### Alternative Options

If automatic installers aren't your cup of tea, check out the [installation
appendix](INSTALL.md) for more options.

## How do I use it?

`ctlptl` supports 4 major commands:

- `ctlptl get` - see all running clusters
- `ctlptl create cluster [product]` - create a cluster and make it the current `kubectl` context
- `ctlptl apply -f cluster.yaml` - ensure a cluster exists, or create one
- `ctlptl delete -f cluster.yaml` - delete a cluster and its state

### Examples

#### Docker for Mac: Enable Kubernetes and set 4 CPU

Create:

```
ctlptl docker-desktop open
ctlptl create cluster docker-desktop --min-cpus=4
```

or ensure exists:

```
cat <<EOF | ctlptl apply -f -
apiVersion: ctlptl.dev/v1alpha1
kind: Cluster
product: docker-desktop
minCPUs: 4
EOF
```

#### Docker for Mac: Reset and shutdown Kubernetes

```
ctlptl delete cluster docker-desktop
ctlptl docker-desktop quit
```

#### KIND: with a built-in registry at a random port

Create:

```
ctlptl create cluster kind --registry=ctlptl-registry
```

or ensure exists:

```
cat <<EOF | ctlptl apply -f -
apiVersion: ctlptl.dev/v1alpha1
kind: Cluster
product: kind
registry: ctlptl-registry
EOF
```

Then fetch the URL to push images to with:

```
ctlptl get cluster kind-kind -o template --template '{{.status.localRegistryHosting.host}}'
```

#### KIND: with a built-in registry at a pre-determined port

Create:

```
ctlptl create registry ctlptl-registry --port=5005
ctlptl create cluster kind --registry=ctlptl-registry
```

or ensure exists:

```
cat <<EOF | ctlptl apply -f -
apiVersion: ctlptl.dev/v1alpha1
kind: Registry
name: ctlptl-registry
port: 5005
---
apiVersion: ctlptl.dev/v1alpha1
kind: Cluster
product: kind
registry: ctlptl-registry
EOF
```

#### K3D: with a built-in registry at a pre-determined port

Create:

```
ctlptl create registry ctlptl-registry --port=5005
ctlptl create cluster k3d --registry=ctlptl-registry
```

or ensure exists:

```
cat <<EOF | ctlptl apply -f -
apiVersion: ctlptl.dev/v1alpha1
kind: Registry
name: ctlptl-registry
port: 5005
---
apiVersion: ctlptl.dev/v1alpha1
kind: Cluster
product: k3d
registry: ctlptl-registry
EOF
```

#### Minikube: with a built-in registry at Kubernetes v1.18.8

Create:

```
ctlptl create cluster minikube --registry=ctlptl-registry --kubernetes-version=v1.18.8
```

or ensure exists:

```
cat <<EOF | ctlptl apply -f -
apiVersion: ctlptl.dev/v1alpha1
kind: Cluster
product: minikube
registry: ctlptl-registry
kubernetesVersion: v1.18.8
EOF
```

#### More

For more details, see:

- Example configurations under [./examples](./examples)
- Complete CLI docs under [./docs](./docs/ctlptl.md)
- Cluster API reference under [pkg.go.dev](https://pkg.go.dev/github.com/tilt-dev/ctlptl/pkg/api#Cluster)

## Why did you make this?

At [Tilt](https://tilt.dev/), we want to make Kubernetes a nice environment for local dev.

We found ourselves spending too much time helping teams debug misconfigurations in their dev environment.

We wrote docs like [Choosing a local dev
cluster](https://docs.tilt.dev/choosing_clusters.html) and example repos like
[kind-local](https://github.com/tilt-dev/kind-local),
[minikube-local](https://github.com/tilt-dev/minikube-local), and
[k3d-local](https://github.com/tilt-dev/k3d-local-registry) to help people get set up.

`ctlptl` is a culmination of what we've learned.

## Features

### Current

- Docker for Mac
- Docker for Windows
- [KIND](https://kind.sigs.k8s.io/) and [KIND with a registry](https://kind.sigs.k8s.io/docs/user/local-registry/)
- [Minikube](https://minikube.sigs.k8s.io/) and Minikube with a registry
- [K3D](https://k3d.io/) with a registry
- Creating a cluster on a Remote Docker Host (useful in CI environments like [CircleCI](https://circleci.com/docs/2.0/building-docker-images/))
- Allocating CPUs

### Future Work

- Microk8s
- Rancher Desktop
- Podman
- Minikube on Hyperkit
- Allocating Memory
- Allocating Storage

## Community

`ctlptl` is a work in progress!

We welcome [contributions](CONTRIBUTING.md) from the Kubernetes community to help make this better.

We expect everyone -- users, contributors, followers, and employees alike -- to abide by our [**Code of Conduct**](CODE_OF_CONDUCT.md).

## Goals

- To support common local cluster setup operations, like create, delete, and reset

- To interoperate well with all local Kubernetes solutions, including `docker-desktop`, `kind`, `minikube`, `k3d`, or `microk8s`

- To connect other resources to a local cluster, like image registries, storage, and CPU/memory

- To help infra engineers manage a consistent dev environment

- To encourage standards that enable interop between devtools, like [KEP 1755](https://github.com/kubernetes/enhancements/tree/master/keps/sig-cluster-lifecycle/generic/1755-communicating-a-local-registry)

## Non-Goals

- `ctlptl` is NOT a Kubernetes setup approach that competes with `kind` or `minikube`, but rather complements these tools.

- `ctlptl` is NOT intended to help you setup a remote cluster, or a remote dev sandbox. If you want to declaratively set up prod clusters, check out [`clusterapi`](https://cluster-api.sigs.k8s.io/).

## Privacy

`ctlptl` sends anonymized usage statistics, so we can improve it on every platform. Opt out with `ctlptl analytics opt out`.

## License

Copyright 2020 Windmill Engineering

Licensed under [the Apache License, Version 2.0](LICENSE)

