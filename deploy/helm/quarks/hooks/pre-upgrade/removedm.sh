#!/bin/bash
set -ex

# https://github.com/cloudfoundry-incubator/quarks-operator/commit/3610c105a75528285ad05303fc7e8963381d3194
kubectl delete quarksjobs.quarks.cloudfoundry.org --ignore-not-found --namespace $SINGLE_NAMESPACE dm