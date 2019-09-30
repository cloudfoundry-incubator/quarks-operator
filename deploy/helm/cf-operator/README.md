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

| Parameter                                         | Description                                                                       | Default                                        |
| ------------------------------------------------- | --------------------------------------------------------------------------------- | ---------------------------------------------- |
| `image.repository`                                | docker hub repository for the cf-operator image                                   | `cf-operator`                                  |
| `image.org`                                       | docker hub organization for the cf-operator image                                 | `cfcontainerization`                           |
| `image.tag`                                       | docker image tag                                                                  | `foobar`                                       |
| `image.pullPolicy`                                | Kubernetes image pullPolicy                                                       | `IfNotPresent`                                 |
| `rbacEnable`                                      | install required RBAC service account, roles and rolebindings                     | `true`                                         |
| `serviceAccount.cfOperatorServiceAccount.create`  | Will set the value of `cf-operator.serviceAccountName` to the current chart name  | `true`                                         |
| `serviceAccount.cfOperatorServiceAccount.name`    | If the above is not set, it will set the `cf-operator.serviceAccountName`         |                                                |

## RBAC

By default, the helm chart will install RBAC ClusterRole and ClusterRoleBinding based on the chart release name, it will also grant the ClusterRole to an specific service account, which have the same name of the chart release.

The RBAC resources are enable by default. To disable:

```bash
helm install --namespace cf-operator --name cf-operator https://s3.amazonaws.com/cf-operators/helm-charts/cf-operator-v0.2.2%2B47.g24492ea.tgz --set rbacEnable=false
```

## Custom Resources

The `cf-operator` watches four different types of custom resources:

- BoshDeployment
- ExtendedJob
- ExtendedSecret
- ExtendedStatefulset

The `cf-operator` requires this CRD´s to be installed in the cluster, in order to work as expected. By default, the `cf-operator` applies CRDs in your cluster automatically.

To verify if the CRD´s are installed:

```bash
$ kubectl get crds
NAME                                            CREATED AT
boshdeployments.fissile.cloudfoundry.org        2019-06-25T07:08:37Z
extendedjobs.fissile.cloudfoundry.org           2019-06-25T07:08:37Z
extendedsecrets.fissile.cloudfoundry.org        2019-06-25T07:08:37Z
extendedstatefulsets.fissile.cloudfoundry.org   2019-06-25T07:08:37Z
```
