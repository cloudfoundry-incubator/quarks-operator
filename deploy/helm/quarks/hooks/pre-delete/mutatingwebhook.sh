#!/bin/bash
set -ex

# https://github.com/cloudfoundry-incubator/quarks-operator/blob/1a6f8b0063455a98df395f6c445e23e1c9e186bd/pkg/kube/controllers/controllers.go#L92
kubectl delete mutatingwebhookconfiguration --ignore-not-found cf-operator-hook-$NAMESPACE
