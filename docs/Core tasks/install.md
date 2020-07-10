---
title: "Install"
linkTitle: "Install quarks-operator"
weight: 1
description: >
  Installation of Quarks-operator in your Kubernetes cluster with helm
---

The `quarks-operator` can be installed via `helm`. You can use our [helm repository](https://cloudfoundry-incubator.github.io/quarks-helm/).

See the [releases page](https://github.com/cloudfoundry-incubator/cf-operator/releases) for up-to-date instructions on how to install the operator.

For more information about the `quarks-operator` helm chart and how to configure it, please refer to the helm repository [README.md](https://github.com/cloudfoundry-incubator/quarks-operator/tree/master/deploy/helm/cf-operator). A short summary of the installation steps is presented below.

## Prerequisites

- Kubernetes cluster
- helm
- kubectl

## Use this if you've never installed the operator before

```bash
helm repo add quarks https://cloudfoundry-incubator.github.io/quarks-helm/
helm install cf-operator quarks/cf-operator
```

## Use this if the custom resources have already been created by a previous CF Operator installation

```bash
helm repo update
helm install cf-operator quarks/cf-operator --set "customResources.enableInstallation=false"
```

## For more options look at the README for the chart


```bash
helm show readme quarks/cf-operator
```

## What next?

With a running `quarks-operator` pod, you can try one of the files (see [boshdeployment-with-custom-variable.yaml](https://raw.githubusercontent.com/cloudfoundry-incubator/quarks-operator/master/docs/examples/bosh-deployment/boshdeployment-with-custom-variable.yaml) ), as follows:

```bash
kubectl -n cf-operator apply -f https://raw.githubusercontent.com/cloudfoundry-incubator/quarks-operator/master/docs/examples/bosh-deployment/boshdeployment-with-custom-variable.yaml
```

The above will spawn two pods in your `cf-operator` namespace (which needs to be created upfront), running the BOSH nats release.

You can access the `cf-operator` logs by following the operator pod's output:

```bash
kubectl logs -f -n cf-operator cf-operator
```

Or look at the k8s event log:

```bash
kubectl get events -n cf-operator --watch
```

## Modifying the deployment

The main input to the operator is the `BOSH deployment` custom resource and the according manifest config map or secret. Changes to the `Spec` or `Data` fields of either of those will trigger the operator to recalculate the desired state and apply the required changes from the current state.

Besides that there are more things the user can change which will trigger an update of the deployment:

* `ops files` can be added or removed from the `BOSH deployment`. Existing `ops file` config maps and secrets can be modified
* generated secrets for [explicit variables](docs/from_bosh_to_kube.md#variables-to-quarks-secrets) can be modified
* secrets for [implicit variables](docs/from_bosh_to_kube.md#manual-implicit-variables) have to be created by the user beforehand anyway, but can also be changed after the initial deployment