#!/bin/bash

# # Changed Labels in quarks-operator v7.1.3
#
# The content of the `quarks.cloudfoundry.org/quarks-statefulset-name` label changed. It now containts the name of the quarks statefulset owning the statefulset and pod. The label is used to create `startup-ordinal` labels that are stable across pod restarts.
#
# A new label `quarks.cloudfoundry.org/statefulset-name` was added. It contains the name of the statefulset, which might include an optional zone suffix (e.g. `-z0`).
# The label is used by the active passive controller to find all the pods of a statefulset.
#
# Code change: https://github.com/cloudfoundry-incubator/quarks-statefulset/blob/master/pkg/kube/controllers/quarksstatefulset/quarksstatefulset_reconciler.go#L247-L268


# Update labels so active-passive works again, this needs to be done for all workload namespaces, but we only support single namespace mode here:
if [ -n "$SINGLE_NAMESPACE" ]; then
  for pod in $(kubectl get pods -n "$SINGLE_NAMESPACE" -o name -l quarks.cloudfoundry.org/deployment-name --ignore-not-found)
  do
      echo 1>&2 "POD $pod ..."

      qsts_name=$(kubectl get -n "$SINGLE_NAMESPACE" "$pod" -o jsonpath="{.metadata.labels.quarks\.cloudfoundry\.org/quarks-statefulset-name}")
      sts_name="$qsts_name"
      qsts_name=$(echo "$qsts_name" | sed -e 's/-z[0-9]\+$//')

      if test -z "${qsts_name}" ; then
        echo 1>&2 "SKIP $pod: empty"
        continue
      fi

      echo 1>&2 "PATCH names=$sts_name/$qsts_name"

      kubectl label --overwrite -n "$SINGLE_NAMESPACE" "$pod" "quarks.cloudfoundry.org/statefulset-name=$sts_name"
      kubectl label --overwrite -n "$SINGLE_NAMESPACE" "$pod" "quarks.cloudfoundry.org/quarks-statefulset-name=$qsts_name"
  done
fi
