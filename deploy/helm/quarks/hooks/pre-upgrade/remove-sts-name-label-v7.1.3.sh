#!/bin/bash

# quarks-operator v7.1.3 added a new label `quarks.cloudfoundry.org/statefulset-name`. This is no longer supported and breaks STS updates.

set -ex

if [ -n "$SINGLE_NAMESPACE" ]; then

  # delete these
  for sts in $(kubectl get sts -n "$SINGLE_NAMESPACE" -o name -l quarks.cloudfoundry.org/statefulset-name --ignore-not-found)
  do
      sts_name=$(echo "$sts" | sed 's@.*/@@')
      echo 1>&2 "### Recreate sts: $sts_name ..."

      if test -z "${sts_name}" ; then
        echo 1>&2 "SKIP STS $sts: error"
        continue
      fi

      kubectl get -n "$SINGLE_NAMESPACE" "$sts" -o json | jq --arg N "$sts_name" '
      del(.metadata.namespace,.metadata.resourceVersion,.metadata.uid,.metadata.managedFields)
      | .metadata.creationTimestamp=null
      | del(
        .spec.template.metadata.labels."quarks.cloudfoundry.org/statefulset-name",
        .metadata.labels."quarks.cloudfoundry.org/statefulset-name",
        .spec.selector.matchLabels."quarks.cloudfoundry.org/statefulset-name")
      | .spec.template.metadata.labels."quarks.cloudfoundry.org/quarks-statefulset-name"=$N
      | .spec.selector.matchLabels."quarks.cloudfoundry.org/quarks-statefulset-name"=$N
      ' > "$sts_name".json

      kubectl delete -n "$SINGLE_NAMESPACE" "$sts" --cascade=orphan --wait=true

      kubectl wait -n "$SINGLE_NAMESPACE" "$sts" --for=delete || true

      kubectl create -n "$SINGLE_NAMESPACE" -f "$sts_name".json
  done

  # # update labels on pods
  # for pod in $(kubectl get pod -n "$SINGLE_NAMESPACE" -o name -l "quarks.cloudfoundry.org/statefulset-name" --ignore-not-found)
  # do
  #   sts_name=$(kubectl get -n "$SINGLE_NAMESPACE" "$pod" -o jsonpath="{.metadata.labels.quarks\.cloudfoundry\.org/statefulset-name}")

  #   echo 1>&2 " ### Update pod labels: $sts"
  #   # delete
  #   kubectl label -n "$SINGLE_NAMESPACE" "$pod" "quarks.cloudfoundry.org/statefulset-name"-
  #   # with az
  #   kubectl label --overwrite -n "$SINGLE_NAMESPACE" "$pod" "quarks.cloudfoundry.org/quarks-statefulset-name=$sts_name"
  # done

fi
