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

See the [releases page](https://github.com/cloudfoundry-incubator/cf-operator/releases) for up-to-date instructions on how to install the operator.

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

## Custom Resources

The `cf-operator` watches four different types of custom resources:

- [BoshDeployment](docs/controllers/boshdeployment.md)
- [ExtendedJob](https://github.com/cloudfoundry-incubator/quarks-job/blob/master/docs/extendedjob.md)
- [ExtendedSecret](docs/controllers/quarks_secret.md)
- [ExtendedStatefulset](docs/controllers/quarks_statefulset.md)

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
The [previous example](docs/examples/bosh-deployment/boshdeployment-with-custom-variable.yaml) creates a secret named `nats-deployment.var-custom-password`. That value will be used to fill `((custom-password))` place holders in the BOSH manifest.

The name of the secret has to follow this scheme: '<bosh-deployment-cr.name>.var-<variable-name>'

Missing implicit variables are treated as an error.

### Explicit Variables

Explicit variables are explicitly defined in the [BOSH manifest](https://bosh.io/docs/manifest-v2/#variables). They are generated automatically upon deployment and stored in secrets.

The naming scheme is the same as for implicit variables.

If an explicit variable secret already exists, it will not be generated. This allows users to set their own passwords, etc.


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

## Development and Tests

For more information about the operator development, see [docs/development.md](docs/development.md)

For more information about testing, see [docs/testing.md](docs/testing.md)

For more information about building the operator from source, see [docs/building.md](docs/building.md)

For more information about how to develop a BOSH release using Quarks and SCF, see the [SCFv3 docs](https://github.com/SUSE/scf/blob/v3-develop/dev/scf/docs/bosh-author.md)
