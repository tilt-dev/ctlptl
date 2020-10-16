# ctlptl

Want to mess around with Kubernetes, but don't want to spend an ocean on
hardware?

Maybe you need a `ctlptl`.

## What is ctlptl?

`ctlptl` (pronounced "control pedal") is a CLI for declaratively setting up
local Kubernetes clusters.

Inspired by `kubectl` and
[ClusterAPI's](https://github.com/kubernetes-sigs/cluster-api) `clusterctl`, you
declare your local cluster with YAML and use `ctlptl` to set it up.

## Why did you make this?

At [Tilt](https://tilt.dev/), we want to make Kubernetes a nice environment for local dev.

We found ourselves spending too much time helping teams debug misconfigurations in their dev environment.

We wrote docs like [Choosing a local dev
cluster](https://docs.tilt.dev/choosing_clusters.html) and example repos like
[kind-local](https://github.com/tilt-dev/kind-local),
[minikube-local](https://github.com/tilt-dev/minikube-local), and
[k3d-local](https://github.com/tilt-dev/k3d-local) to help people get set up.

`ctlptl` is a culmination of what we've learned.

## How do I try it?

We're still writing it! Stay tuned.

## License

Copyright 2020 Windmill Engineering

Licensed under [the Apache License, Version 2.0](LICENSE)

