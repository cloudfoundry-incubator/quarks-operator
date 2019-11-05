# QUARKS cf-operator

## Introduction

This helm chart deploys the cf-operator, which allow the deployment of a bosh manifest through a configmap and watches for changes on it.

## Installing the latest stable chart

To install the latest stable helm chart, with the `cf-operator` as the release name and namespace:

```bash
helm install --namespace cf-operator --name cf-operator https://s3.amazonaws.com/cf-operators/helm-charts/cf-operator-v0.2.2%2B47.g24492ea.tgz
```

## Installing the chart from develop branch

To install the helm chart directly from the [cf-operator repository](https://github.com/cloudfoundry-incubator/cf-operator) (any branch), the following parameters in the `values.yaml` need to be set in advance:

| Parameter                                         | Description                                                          | Default                                        |
| ------------------------------------------------- | -------------------------------------------------------------------- | ---------------------------------------------- |
| `image.repository`                                | docker hub repository for the cf-operator image                      | `cf-operator`                                  |
| `image.org`                                       | docker hub organization for the cf-operator image                    | `cfcontainerization`                           |
| `image.tag`                                       | docker image tag                                                     | `foobar`                                       |

### For a local development with minikube, you can generate the image first and then use the `$VERSION_TAG` environment variable into the `image.tag`:

```bash
export GO111MODULE=on
eval `minikube docker-env`
. bin/include/versioning
echo "Tag for docker image is ${VERSION_TAG}"
./bin/build-image
```

Either set the `image.tag` in the `values.yaml`, or pass the `$VERSION_TAG` to `helm install`:

```bash
helm install deploy/helm/cf-operator/ --namespace cf-operator --name cf-operator --set image.tag=$VERSION_TAG
```

### For a local development with minikube and havener

Make sure you have [havener](https://github.com/homeport/havener) install.

```bash
havener deploy --config dev-env-havener.yaml
```

## Uninstalling the chart

To delete the helm chart:

```bash
helm delete cf-operator --purge
```

## Configuration

| Parameter                                         | Description                                                                          | Default                                        |
| ------------------------------------------------- | ------------------------------------------------------------------------------------ | ---------------------------------------------- |
| `image.repository`                                | Docker hub repository for the cf-operator image                                      | `cf-operator`                                  |
| `image.org`                                       | Docker hub organization for the cf-operator image                                    | `cfcontainerization`                           |
| `image.tag`                                       | Docker image tag                                                                     | `foobar`                                       |
| `global.contextTimeout`                           | Will set the context timeout in seconds, for future K8S API requests                 | `30`                                           |
| `global.image.pullPolicy`                         | Kubernetes image pullPolicy                                                          | `IfNotPresent`                                 |
| `global.operator.watchNamespace`                  | Namespace the operator will watch for BOSH deployments                               | the release namespace                          |
| `global.rbacEnable`                               | Install required RBAC service account, roles and rolebindings                        | `true`                                         |
| `operator.webhook.endpoint`                       | Hostname/IP under which the webhook server can be reached from the cluster           | the IP of service `cf-operator-webhook `       |
| `operator.webhook.port`                           | Port the webhook server listens on                                                   | 2999                                           |
| `operator.webhook.useServiceReference`            | If true, the webhook server is addressed using a service reference instead of the IP | `true`                                         |
| `serviceAccount.cfOperatorServiceAccount.create`  | Will set the value of `cf-operator.serviceAccountName` to the current chart name     | `true`                                         |
| `serviceAccount.cfOperatorServiceAccount.name`    | If the above is not set, it will set the `cf-operator.serviceAccountName`            |                                                |

> **Note:**
>
> `operator.webhook.useServiceReference` will override `operator.webhook.endpoint` configuration
>

## Watched namespace

By default the operator will watch for BOSH deployments in the same namespace as it has been deployed to. Optionally, the watched namespace can be changed to something else using the `global.operator.watchNamespace` value, e.g.

```bash
$ helm install --namespace cf-operator --name cf-operator https://s3.amazonaws.com/cf-operators/helm-charts/cf-operator-v0.2.2%2B47.g24492ea.tgz --set global.operator.watchNamespace=staging
```

## RBAC

By default, the helm chart will install RBAC ClusterRole and ClusterRoleBinding based on the chart release name, it will also grant the ClusterRole to an specific service account, which have the same name of the chart release.

The RBAC resources are enable by default. To disable:

```bash
helm install --namespace cf-operator --name cf-operator https://s3.amazonaws.com/cf-operators/helm-charts/cf-operator-v0.2.2%2B47.g24492ea.tgz --set global.rbacEnable=false
```
