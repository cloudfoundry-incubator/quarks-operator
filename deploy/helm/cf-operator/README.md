# QUARKS cf-operator

## Introduction

This helm chart deploys the cf-operator, which allow the deployment of a bosh manifest through a configmap and watches for changes on it.

## Installing the Latest Stable Chart

To install the latest stable helm chart, with the `cf-operator` as the release name and namespace:

```bash
helm repo add quarks https://cloudfoundry-incubator.github.io/quarks-helm/
helm install cf-operator quarks/cf-operator
```

## Installing the Chart From the Development Branch

Run `bin/build-image` to create a new docker image, export `DOCKER_IMAGE_TAG` to override the tag.

To install the helm chart directly from the [cf-operator repository](https://github.com/cloudfoundry-incubator/cf-operator) (any branch), run `bin/build-helm` first.

## Uninstalling the Chart

To delete the helm chart:

```bash
helm delete cf-operator
```

## Configuration

| Parameter                                         | Description                                                                                       | Default                                        |
| ------------------------------------------------- | ------------------------------------------------------------------------------------------------- | ---------------------------------------------- |
| `image.repository`                                | Docker hub repository for the cf-operator image                                                   | `cf-operator`                                  |
| `image.org`                                       | Docker hub organization for the cf-operator image                                                 | `cfcontainerization`                           |
| `image.tag`                                       | Docker image tag                                                                                  | `foobar`                                       |
| `createWatchNamespace`                            | Create the namespace, which is used for deployment                                                | `true`                                         |
| `global.contextTimeout`                           | Will set the context timeout in seconds, for future K8S API requests                              | `300`                                           |
| `global.image.pullPolicy`                         | Kubernetes image pullPolicy                                                                       | `IfNotPresent`                                 |
| `global.image.credentials`                        | Kubernetes image pull secret credentials (map with keys `servername`, `username`, and `password`) | `nil`                                          |
| `global.operator.watchNamespace`                  | Namespace the operator will watch for BOSH deployments                                            | the release namespace                          |
| `global.rbac.create`                              | Install required RBAC service account, roles and rolebindings                                     | `true`                                         |
| `operator.webhook.endpoint`                       | Hostname/IP under which the webhook server can be reached from the cluster                        | the IP of service `cf-operator-webhook`        |
| `operator.webhook.port`                           | Port the webhook server listens on                                                                | 2999                                           |
| `global.operator.webhook.useServiceReference`     | If true, the webhook server is addressed using a service reference instead of the IP              | `true`                                         |
| `serviceAccount.create`                           | If true, create a service account                                                                 | `true`                                         |
| `serviceAccount.name`                             | If not set and `create` is `true`, a name is generated using the fullname of the chart            |                                                |

> **Note:**
>
> `global.operator.webhook.useServiceReference` will override `operator.webhook.endpoint` configuration
>

## Watched Namespace

The operator will watch for BOSH deployments in a separate namespace, not the one it has been deployed to. The watched namespace can be changed to something else using the `global.operator.watchNamespace` value, e.g.

```bash
$ helm install cf-operator quarks/cf-operator --namespace cf-operator --set global.operator.watchNamespace=staging
```

## RBAC

By default, the helm chart will install RBAC ClusterRole and ClusterRoleBinding based on the chart release name, it will also grant the ClusterRole to an specific service account, which have the same name of the chart release.

The RBAC resources are enable by default. To disable:

```bash
helm install cf-operator quarks/cf-operator --namespace cf-operator --set global.rbac.create=false
```
