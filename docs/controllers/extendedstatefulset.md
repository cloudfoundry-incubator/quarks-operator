# ExtendedStatefulSet

- [ExtendedStatefulSet](#extendedstatefulset)
  - [Description](#description)
  - [Features](#features)
    - [Scaling Restrictions](#scaling-restrictions)
    - [Automatic Restart of Containers](#automatic-restart-of-containers)
    - [Extended Upgrade Support](#extended-upgrade-support)
    - [Annotated if Stale](#annotated-if-stale)
    - [Detect if StatefulSet versions are running](#detect-if-statefulset-versions-are-running)
    - [Volume Management](#volume-management)
    - [AZ Support](#az-support)
  - [Example Resource](#example-resource)

## Description

## Features

### Scaling Restrictions

Ability to set restrictions on how scaling can occur: min, max, odd replicas.

### Automatic Restart of Containers

When an env value or mount changes due to a `ConfigMap` or `Secret` change, containers are restarted.
The operator watches all the ConfigMaps and Secrets referenced by the StatefulSet, and automatically performs the update, without extra workarounds.

> See [this implementation](https://thenewstack.io/solving-kubernetes-configuration-woes-with-a-custom-controller/) for inspiration

Adding an OwnerReference to all ConfigMaps and Secrets that are referenced by an ExtendedStatefulSet. 

```yaml
apiVersion: v1
  data:
    key1: value1
  kind: ConfigMap
  metadata:
    name: example-config
    namespace: default
    ownerReferences:
    - apiVersion: fissile.cloudfoundry.org/v1alpha1
      blockOwnerDeletion: true
      controller: false
      kind: ExtendedStatefulSet
      name: example-extendedstatefulset
```

This allows Controller to trigger a reconciliation whenever the ConfigMaps or Secrets are modified.

And Controller managed ExtendedStatefulSet and StatefulSets will have a `fissile.cloudfoundry.org/finalizer` Finalizer. This allows Controller to perform additional cleanup logic which prevents owned ConfigMaps and Secrets from being deleted.

```yaml
apiVersion: fissile.cloudfoundry.org/v1alpha1
kind: ExtendedStatefulSet
metadata:
  finalizers:
  - fissile.cloudfoundry.org/finalizer
  generation: 1
  name:  example-extended-stateful-set
  namespace: default
```

### Extended Upgrade Support

A second StatefulSet for the new version is deployed, and both coexist until canary conditions are met. This also allows support for Blue/Green techniques.

> Note: This could make integration with Istio easier and (more) seamless.

Annotated with a version (auto-incremented on each update).

Ability to upgrade even though StatefulSet pods are not ready.

An ability to run an `ExtendedJob` before and after the upgrade. The Job can abort the upgrade if it doesn't complete successfully.

### Annotated if Stale

If a failure has occurred (e.g. canary has failed), the StatefulSet is annotated as being stale.

### Detect if StatefulSet versions are running

During upgrades, there is more than one `StatefulSet` version for an `ExtendedStatefulSet` resource.
The operator can list available versions and store status that keeps track of which is running:

```yaml
status:
  versions:
    "1": true
    "2": false

```

A version running means that at least one pod that belongs to the `StatefulSet` is running.
When a version is running, any version lower than it is deleted.

```yaml
status:
  versions:
    # version 1 was cleaned up
    "2": true
```

Controller will continue to reconcile until there's only one version.

### Volume Management

![Volume Claim management across versions](https://docs.google.com/drawings/d/e/2PACX-1vSvQkXe3zZhJYbkVX01mxS4PKa1iQmWyIgdZh1VKtTS1XW1lC14d1_FHLWn2oA7GVgzJCcEorNVXkK_/pub?w=1185&h=1203)

### AZ Support

TODO

## Example Resource

```yaml
---
apiVersion: fissile.cloudfoundry.org/v1alpha1
kind: ExtendedStatefulSet
metadata:
  name: MyExtendedStatefulSet
spec:
  az:  
  scaling:
    # Minimum replica count for the StatefulSet
    min: 3
    # Maximum replica count for the StatefulSet
    max: 13
    # If true, only odd replica counts are valid when scaling the StatefulSet
    oddOnly: true
  # If true, the StatefulSet will be updated When an env value or mount changes
  updateOnEnvChange: true
  updateStrategy:
    canaries: 1
    retryCount: 2
    updateNotReady: true

  # Below you can see a template for a regular StatefulSet
  # Nothing else is custom below this point
  template:
    metadata:
      labels:
        a-label-for-my-stateful-set: "foo"
    spec:
      selector:
        matchLabels:
          app: my-app
      serviceName: "app"
      template:
        metadata:
          labels:
            app: my-app
        spec:
          terminationGracePeriodSeconds: 10
          containers:
          - name: my-web-app
            image: k8s.gcr.io/nginx-slim:0.8
            ports:
            - containerPort: 80
              name: web
```
