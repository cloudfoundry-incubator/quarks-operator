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

A working kubernetes cluster.

## Install

The alpha release of cf-operator has to be built from source. The helm chart is not working yet.

    git checkout v0.1.0

We need to build the binary, which can run out of cluster:

    bin/build

When starting the operator it needs to be reachable from the kubernetes API, so the web hooks work.

    export CF_OPERATOR_WEBHOOK_SERVICE_HOST=<your-public-ip>

We need to tell the operator which docker image it can use for template rendering:

    export DOCKER_IMAGE_TAG=v0.1.0
    export OPERATOR_DOCKER_ORGANIZATION=cfcontainerization

For template rendering the operators docker image needs to be accessible to the cluster:

    docker build . -t $DOCKER_ORGANIZATION/cf-operator:$DOCKER_IMAGE_TAG

Apply the custom resource definitions to the cluster:

    bin/apply-crds

Optionally create a namespace for the operator to work in:

    kubectl create namespace test
    export CF_OPERATOR_NAMESPACE=test

Finally run the operator

    binaries/cf-operator


## Development

For more information see [docs/development.md](docs/development.md) and [docs/testing.md](docs/testing.md)

### Requirements

Go 1.12.2 and install the tool chain:

    make tools

### Dependencies

Run with libraries fetched via go modules:

    export GO111MODULE=on

Or with a vendor folder, using GO111MODULE=off, this also speeds up docker builds

    export GO111MODULE=off
    go mod vendor

Or by checking out the versioned vendor git sub module

    git submodule update --init

### Prepare

Setup environment variables, most importantly `CF_OPERATOR_WEBHOOK_SERVICE_HOST`.

    # set to IP reachable from k8s API, e.g. on Linux:
    export CF_OPERATOR_WEBHOOK_SERVICE_HOST=$(ip -4 a s dev `ip r l 0/0 | tail -1 | cut -f5 -d' '` | grep -oP 'inet \K\S+(?=/)')

    # optionally, if using minikube, build the image directly on minikube's docker
    # eval `minikube docker-env`

### Start Operator Locally

This will build the image and tag it with `$DOCKER_IMAGE_TAG`. If not set, the version will be calculated from Git information.
Afterwards the operator is started out-of-cluster.

    make up

Build and use the helm charts to run the operator in-cluster.

### Run Integration tests

See [docs/testing.md](docs/testing.md#Integration)

### Test with Example Data
    kubectl apply -f docs/examples/fissile_v1alpha1_boshdeployment_cr.yaml
    kubectl get boshdeployments.fissile.cloudfoundry.org
    kubectl get pods --watch

    # clean up
    kubectl delete configmap bosh-manifest
    kubectl delete configmap bosh-ops
    kubectl delete secret bosh-ops-secret
    kubectl delete boshdeployments.fissile.cloudfoundry.org example-boshdeployment
