# cf-operator

[![godoc](https://godoc.org/code.cloudfoundry.org/cf-operator?status.svg)](https://godoc.org/code.cloudfoundry.org/cf-operator)
[![master](https://ci.flintstone.cf.cloud.ibm.com/api/v1/teams/quarks/pipelines/cf-operator/badge)](https://ci.flintstone.cf.cloud.ibm.com/teams/quarks/pipelines/cf-operator)
[![go report card](https://goreportcard.com/badge/code.cloudfoundry.org/cf-operator)](https://goreportcard.com/report/code.cloudfoundry.org/cf-operator)
[![Coveralls github](https://img.shields.io/coveralls/github/cloudfoundry-incubator/cf-operator.svg?style=flat)](https://coveralls.io/github/cloudfoundry-incubator/cf-operator?branch=HEAD)

|Nightly build|[![nightly](https://ci.flintstone.cf.cloud.ibm.com/api/v1/teams/quarks/pipelines/cf-operator-nightly/badge)](https://ci.flintstone.cf.cloud.ibm.com/teams/quarks/pipelines/cf-operator-nightly)|
|-|-|

<img align="right" width="200" height="39" src="https://github.com/cloudfoundry-incubator/cf-operator/raw/master/docs/cf-operator-logo.png">

cf-operator enables the deployment of BOSH Releases, especially Cloud Foundry, to Kubernetes.

It's implemented as a k8s operator, an active controller component which acts upon custom k8s resources.

* Incubation Proposal: [Containerizing Cloud Foundry](https://docs.google.com/document/d/1_IvFf-cCR4_Hxg-L7Z_R51EKhZfBqlprrs5NgC2iO2w/edit#heading=h.lybtsdyh8res)
* Slack: #quarks-dev on <https://slack.cloudfoundry.org>
* Backlog: [Pivotal Tracker](https://www.pivotaltracker.com/n/projects/2192232)
* Docker: https://hub.docker.com/r/cfcontainerization/cf-operator/tags

## Installing

### **Using the helm chart**

The `cf-operator` can be installed via `helm`. Make sure you have a running Kubernetes cluster and that tiller is reachable.

Use this if you've never installed the operator before

```bash
helm install --namespace cf-operator --name cf-operator https://cf-operators.s3.amazonaws.com/release/helm-charts/cf-operator-v0.4.0%2B1.g3d277af0.tgz
```

Use this if the custom resources have already been created by a previous CF Operator installation

```bash
helm install --namespace cf-operator --name cf-operator https://cf-operators.s3.amazonaws.com/release/helm-charts/cf-operator-v0.4.0%2B1.g3d277af0.tgz --set "customResources.enableInstallation=false"
````

For more information about the `cf-operator` helm chart and how to configure it, please refer to [deploy/helm/cf-operator/README.md](deploy/helm/cf-operator/README.md)

### Recovering from a crash

If the operator pod crashes, it cannot be restarted in the same namespace before the existing mutating webhook configuration for that namespace is removed.
The operator uses mutating webhooks to modify pods on the fly and Kubernetes fails to create pods if the webhook server is unreachable.
The webhook configurations are installed cluster wide and don't belong to a single namespace, just like custom resources.

To remove the webhook configurations for the cf-operator namespace run:

```bash
CF_OPERATOR_NAMESPACE=cf-operator
kubectl delete mutatingwebhookconfiguration "cf-operator-hook-$CF_OPERATOR_NAMESPACE"
kubectl delete validatingwebhookconfiguration "cf-operator-hook-$CF_OPERATOR_NAMESPACE"
```

## Using your fresh installation

With a running `cf-operator` pod, you can try one of the files (see [docs/examples/bosh-deployment/boshdeployment-with-custom-variable.yaml](docs/examples/bosh-deployment/boshdeployment-with-custom-variable.yaml) ), as follows:

```bash
kubectl -n cf-operator create -f docs/examples/bosh-deployment/boshdeployment-with-custom-variable.yaml
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

For now deployments have to be in the namespace the operator is running in.

## Development and Tests

For more information about the operator development, see [docs/development.md](docs/development.md)

For more information about testing, see [docs/testing.md](docs/testing.md)

For more information about building the operator from source, see [docs/building.md](docs/building.md)

For more information about how to develop a BOSH release using Quarks and SCF, see the [SCFv3 docs](https://github.com/SUSE/scf/blob/v3-develop/dev/scf/docs/bosh-author.md)
