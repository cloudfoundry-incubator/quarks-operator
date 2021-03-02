# QUARKS Operator

## Introduction

This helm chart deploys the quarks-operator, which allow the deployment of a bosh manifest through a configmap and watches for changes on it.

The Quarks-operator documentation is available at: https://quarks.suse.dev/docs/

For notes about the installation, see the relevant section: https://quarks.suse.dev/docs/quarks-operator/install/

## Installing the Latest Stable Chart

To install the latest stable helm chart, with the `quarks-operator` as the release name and namespace:

```bash
helm repo add quarks https://cloudfoundry-incubator.github.io/quarks-helm/
helm install qops1 quarks/quarks
```

The operator will watch for BOSH deployments in separate namespaces (default: one namespace named 'staging'), not the one it has been deployed to.

### Using multiple operators

Choose different namespaces and cluster role names. The persist output service account will be named the same as the cluster role as well as for coredns:

```
helm install qops1 quarks/quarks \
  --namespace namespace1
  --set "global.singleNamespace.name=staging1" \
  --set "global.monitoredID=id1" \
  --set "quarks-job.persistOutputClusterRole.name=clusterrole1" \
  --set  "corednsServiceAccount.name=clusterrole2" \
```

### Using multiple namespaces with one operator

The cluster role can be reused between namespaces.
The service account (and role binding) should be different for each namespace.

```
helm install relname1 quarks/quarks \
  --set "global.singleNamespace.create=false"
```

Manually create before running `helm install`, for each namespace:

* a namespace "staging1" with the following labels (note: "cfo", "qjob-persist-output" and "coredns-quarks-service-account" are the defaults from `values.yaml`):
  * quarks.cloudfoundry.org/monitored: cfo
  * quarks.cloudfoundry.org/qjob-service-account: qjob-persist-output
  * quarks.cloudfoundry.org/coredns-quarks-service-account: coredns-quarks
* a service account named "qjob-persist-output" and "coredns-quarks"
* a role binding from the existing cluster role "qjob-persist-output" to "qjob-persist-output" service account in namespace "staging1"
* another cluster binding from the existing cluster role "coredns-quarks" to "coredns-quarks" service account in namesapce "staging1"

## Installing the Chart From the Development Branch

Download the shared scripts with `bin/tools`, set `PROJECT=quarks-operator` and run `bin/build-image` to create a new docker image, export `DOCKER_IMAGE_TAG` to override the tag.

To install the helm chart directly from the [quarks repository](https://github.com/cloudfoundry-incubator/quarks-operator) (any branch), run `bin/build-helm` first.

## Uninstalling the Chart

To delete the helm chart:

```bash
helm delete qops1
```

## Configuration

For more possible parameters look in [`values.yml`](https://github.com/cloudfoundry-incubator/quarks-operator/blob/master/deploy/helm/quarks/values.yaml).

| Parameter                                         | Description                                                                                       | Default                                        |
| ------------------------------------------------- | ------------------------------------------------------------------------------------------------- | ---------------------------------------------- |
| `image.repository`                                | Docker hub repository for the quarks-operator image                                                   | `quarks-operator`                                  |
| `image.org`                                       | Docker hub organization for the quarks-operator image                                                 | `cfcontainerization`                           |
| `image.tag`                                       | Docker image tag                                                                                  | `foobar`                                       |
| `logrotateInterval`                               | Logrotate interval in minutes                                                                     | `1440`                                         |
| `logLevel`                                        | Only show log messages which are at least at the given level (trace,debug,info,warn)              | `debug`                                        |
| `global.contextTimeout`                           | Will set the context timeout in seconds, for future K8S API requests                              | `300`                                          |
| `global.image.pullPolicy`                         | Kubernetes image pullPolicy                                                                       | `IfNotPresent`                                 |
| `global.image.credentials`                        | Kubernetes image pull secret credentials (map with keys `servername`, `username`, and `password`) | `nil`                                          |
| `global.monitoredID`                              | Label value of 'quarks.cloudfoundry.org/monitored'. Only matching namespaces are watched          | `cfo`                                          |
| `global.rbac.create`                              | Install required RBAC service account, roles and rolebindings                                     | `true`                                         |
| `operator.webhook.endpoint`                       | Hostname/IP under which the webhook server can be reached from the cluster                        | the IP of service `cf-operator-webhook`        |
| `operator.webhook.port`                           | Port the webhook server listens on                                                                | 2999                                           |
| `global.operator.webhook.useServiceReference`     | If true, the webhook server is addressed using a service reference instead of the IP              | `true`                                         |
| `serviceAccount.create`                           | If true, create a service account                                                                 | `true`                                         |
| `serviceAccount.name`                             | If not set and `create` is `true`, a name is generated using the name of the chart                |                                                |
| `global.singleNamespace.create`                   | If true, create a service account and a single watch namespace                                    | `true`                                         |
| `global.singleNamespace.name`                     | Name of the single watch namespace, that will be watched for BOSH deployment                      | `staging`                                      |
| `applyCRD`              | If True, the quarks-operator will install the CRD's.                                                                        | `true`
|
> **Note:**
>
> `global.operator.webhook.useServiceReference` will override `operator.webhook.endpoint` configuration
>

## RBAC

By default, the helm chart will install RBAC ClusterRole and ClusterRoleBinding based on the chart release name, it will also grant the ClusterRole to an specific service account, which have the same name of the chart release.

The RBAC resources are enable by default. To disable:

```bash
helm install qops1 quarks/quarks --namespace qops1 --set global.rbac.create=false
```
