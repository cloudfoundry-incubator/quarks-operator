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

## Requirements

- A working Kubernetes cluster
- Helm binary
- Go 1.12.2 and install the tool chain: `make tools`


## Dependencies

Run with libraries fetched via go modules:

```bash
export GO111MODULE=on
```

## Installing

### **Using the helm chart**

The `cf-operator` can be installed via `helm`. Make sure you have a running Kubernetes cluster and that tiller is reachable.

Use this if you've never installed the operator before

```bash
helm install --namespace cf-operator --name cf-operator https://s3.amazonaws.com/cf-operators/helm-charts/cf-operator-v0.3.0%2B1.g551e559.tgz
```

Use this if the custom resources have already been created by a previous CF Operator installation

```bash
helm install --namespace cf-operator --name cf-operator https://s3.amazonaws.com/cf-operators/helm-charts/cf-operator-v0.3.0%2B1.g551e559.tgz --set "customResources.enableInstallation=false"
````

For more information about the `cf-operator` helm chart and how to configure it, please refer to [deploy/helm/cf-operator/README.md](deploy/helm/cf-operator/README.md)

### **Build it from source**

Follow this steps to build a proper docker image and generate a deployable helm chart:

1. Checkout the latest stable release / or run it from develop branch

    ```bash
    git checkout v0.3.0
    ```

2. Build the cf-operator binary, this will be embedded later on the docker image

    ```bash
    bin/build
    ```

3. Build the docker image

    When running in minikube, please run: `eval $(minikube docker-env)`, to build the image
    directly on minikube docker.

    ```bash
    bin/build-image
    ```

    _**Note**_: This will automatically generate a docker image tag based on your current commit, tag and SHA.

4. Apply Kubernetes Custom Resources

    ```bash
    bin/apply-crds
    ```

5. Generated helm charts with a proper docker image tag, org and repository

    ```bash
    bin/build-helm
    ```

    _**Note**_: This will generate a new directory under the base dir, named `helm/`

6. Install the helm chart

    ```bash
    helm install --name cf-operator-test --namespace cf-operator-test helm/cf-operator --set customResources.enableInstallation=false
    ```

    _**Note**_: The cf-operator will be available under the `cf-operator-test` namespace, running as a pod.

## Using your fresh installation

With a running `cf-operator` pod, you can try one of the files (see [docs/examples/bosh-deployment/boshdeployment-with-custom-variable.yaml](docs/examples/bosh-deployment/boshdeployment-with-custom-variable.yaml) ), as follows:

```bash
kubectl -n cf-operator-test create -f docs/examples/bosh-deployment/boshdeployment-with-custom-variable.yaml
```

The above will spam two pods in your `cf-operator-test` namespace, running the BOSH nats release.

## Development

For more information see [docs/development.md](docs/development.md) and [docs/testing.md](docs/testing.md)

## Running Tests

See [docs/testing.md](docs/testing.md)
