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

## Features

cf-operator deploys dockerized BOSH releases onto existing Kubernetes cluster

* Supports operations files to modify manifest
* Service instance groups become pods, each job in one container
* Errand instance groups become QuarksJobs

To do this it relies on three Kubernetes components:

* QuarksSecret, a custom resource and controller for the generation and rotation of secrets
* [QuarksJob](https://github.com/cloudfoundry-incubator/quarks-job), templating for Kubernetes jobs, which can trigger jobs on configuration changes and persist their output to secrets
* QuarksStatefulSet, adds canary, zero-downtime deployment, zones and active-passive probe support

The cf-operator supports RBAC and uses immutable, versioned secrets internally.

## Installing

### **Using the helm chart**

The `cf-operator` can be installed via `helm`. You can use our [helm repository](https://cloudfoundry-incubator.github.io/quarks-helm/).

See the [releases page](https://github.com/cloudfoundry-incubator/cf-operator/releases) for up-to-date instructions on how to install the operator.

For more information about the `cf-operator` helm chart and how to configure it, please refer to [deploy/helm/cf-operator/README.md](deploy/helm/cf-operator/README.md)

## Using your fresh installation

With a running `cf-operator` pod, you can try one of the files (see [docs/examples/bosh-deployment/boshdeployment-with-custom-variable.yaml](docs/examples/bosh-deployment/boshdeployment-with-custom-variable.yaml) ), as follows:

```bash
kubectl -n cf-operator apply -f docs/examples/bosh-deployment/boshdeployment-with-custom-variable.yaml
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

The cf-operator supports one deployment per namespace.

## Custom Resources

The `cf-operator` watches four different types of custom resources:

* [BoshDeployment](docs/controllers/bosh_deployment.md)
* [QuarksJob](https://github.com/cloudfoundry-incubator/quarks-job/blob/master/docs/quarksjob.md)
* [QuarksSecret](docs/controllers/quarks_secret.md)
* [QuarksStatefulSet](docs/controllers/quarks_statefulset.md)

The `cf-operator` requires the according CRDs to be installed in the cluster in order to work as expected. By default, the `cf-operator` applies CRDs in your cluster automatically.

To verify that the CRD´s are installed:

```bash
$ kubectl get crds
NAME                                            CREATED AT
boshdeployments.quarks.cloudfoundry.org        2019-06-25T07:08:37Z
quarksjobs.quarks.cloudfoundry.org           2019-06-25T07:08:37Z
quarkssecrets.quarks.cloudfoundry.org        2019-06-25T07:08:37Z
quarksstatefulsets.quarks.cloudfoundry.org   2019-06-25T07:08:37Z
```

## Variables

BOSH releases consume two types of variables, explicit and implicit ones.

### Implicit Variables

Implicit variables have to be created before creating a BOSH deployment resource.
The [previous example](docs/examples/bosh-deployment/boshdeployment-with-custom-variable.yaml) creates a secret named `var-custom-password`. That value will be used to fill `((custom-password))` place holders in the BOSH manifest.

The name of the secret has to follow this scheme: 'var-<variable-name>'

Missing implicit variables are treated as an error.

### Explicit Variables

Explicit variables are explicitly defined in the [BOSH manifest](https://bosh.io/docs/manifest-v2/#variables). They are generated automatically upon deployment and stored in secrets.

The naming scheme is the same as for implicit variables.

If an explicit variable secret already exists, it will not be generated. This allows users to set their own passwords, etc.

## Compatibility with BOSH

* Supports BOSH deployment manifests, including links and addons
* Uses available BPM information from job releases
* Renders ERB job templates in an init container, before starting the dockerized BOSH release
* Adds endpoints and services for instance groups
* BOSH DNS support
* Uses Kubernetes zones for BOSH AZs
* Interaction with configuration:
  * BOSH links can be provided by existing Kubernetes secrets
  * Provides BOSH link properties as Kubernetes secrets
  * Generates explicit variables, e.g. password, certificate, and SSH keys
  * Reads implicit variables from secrets
  * Secret rotation for individual secrets

* Adapting releases:
  * Pre-render scripts to patch releases, which are incompatible with Kubernetes

* Lifecycle related:
  * Restart only affected instance groups on update
  * Sequential startup of instance groups
  * Kubernetes healthchecks instead of monit

## Troubleshooting

### Cluster CA

The `cf-operator` assumes that the cluster root CA is also used for signing CSRs via the certificates.k8s.io API and will embed this CA in the generated certificate secrets. If your cluster is set up to use a different cluster-signing CA the generated certificates will have the wrong CA embedded. See https://kubernetes.io/docs/tasks/tls/managing-tls-in-a-cluster/ for more information on cluster trust.

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

From **Kubernetes 1.15** onwards, it is possible to instead patch the webhook configurations for the cf-operator namespace via:

```bash
CF_OPERATOR_NAMESPACE=cf-operator
kubectl patch mutatingwebhookconfigurations "cf-operator-hook-$CF_OPERATOR_NAMESPACE" -p '
webhooks:
- name: mutate-pods.quarks.cloudfoundry.org
  objectSelector:
    matchExpressions:
    - key: name
      operator: NotIn
      values:
      - "cf-operator"
'
```

## Nice tools to use

The following is a list of tools with their respective main features that can help you
to simplify your development work when dealing with [cf-operator](https://github.com/cloudfoundry-incubator/cf-operator) and [kubecf](https://github.com/SUSE/kubecf)

### [k9s](https://github.com/derailed/k9s)

It provides an easy way to navigate through your k8s resources, while watching lively
to changes on them. Main features that can be helpful for containerized CF are:

* inmediate access to resources YAMLs definition

* inmediate access to services endpoints

* inmediate access to pods/container logs

* sort resources(e.g. pods) by cpu or memory consumption

* inmediate access to a container secure shell

### [havener](https://github.com/homeport/havener)

A tool-kit with different features around k8s and CloudFoundry

* `top`, to get an overview on the cpu/memory/load of the cluster, per ns and pods.

* `logs`, to download all logs from all pods into your local system

* `pod-exec`, to open a shell into containers. This can execute cmds in different containers
simultaneously.

* `node-exec`, to open a shell into nodes. This can execute cmds in different containers
simultaneously.

### [stern](https://github.com/wercker/stern)

Allows you to tail multiple pods on k8s and multiple containers within the pod.

### [kube dashboard](https://kubernetes.io/docs/tasks/access-application-cluster/web-ui-dashboard/)

A more user friendly to navigate your k8s cluster resources.

## Development and Tests

Also see [CONTRIBUTING.md](CONTRIBUTING.md).

For more information about

* the operator development, see [docs/development.md](docs/development.md)
* testing, see [docs/testing.md](docs/testing.md)
* building the operator from source, see [docs/building.md](docs/building.md)
* how to develop a BOSH release using Quarks and SCF, see the [SCFv3 docs](https://github.com/SUSE/scf/blob/v3-develop/dev/scf/docs/bosh-author.md)

