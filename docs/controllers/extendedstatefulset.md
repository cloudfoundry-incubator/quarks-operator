# ExtendedStatefulSet

- [ExtendedStatefulSet](#extendedstatefulset)
  - [Description](#description)
  - [Features](#features)
    - [Scaling Restrictions](#scaling-restrictions)
    - [Automatic Restart of Containers](#automatic-restart-of-containers)
    - [Extended Upgrade Support](#extended-upgrade-support)
    - [Annotated if Stale](#annotated-if-stale)
  - [Example Resource](#example-resource)

## Description

## Features

### Scaling Restrictions

Ability to set restrictions on how scaling can occur: min, max, odd replicas.

### Automatic Restart of Containers

When an env value or mount changes due to a `ConfigMap` or `Secret` change, containers are restarted.
The operator watches all the ConfigMaps and Secrets referenced by the StatefulSet, and automatically performs the update, without extra workarounds.

> See [this implementation](https://thenewstack.io/solving-kubernetes-configuration-woes-with-a-custom-controller/) for inspiration

Adding an OwnerReference to all ConfigMaps and Secrets that are referenced by a StatefulSet. 

```yaml
apiVersion: v1
  data:
    key1: value1
  kind: ConfigMap
  metadata:
    name: example-config
    namespace: default
    ownerReferences:
    - apiVersion: apps/v1
      blockOwnerDeletion: true
      controller: false
      kind: StatefuelSet
      name: example-stateful-set
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

A second StatefulSet for the new version is deployed, and both coexist until canary conditions are met. This also allows support for Blue/Green tehniques. 

> Note: This could make integration with Istio easier and (more) seamless.

Annotated with a version (auto-incremented on each update). 

Ability to upgrade even though StatefulSet pods are not ready.

An ability to run an `ExtendedJob` before and after the upgrade. The Job can abort the upgrade if it doesn't complete successfully.

### Annotated if Stale

If a failure has occurred (e.g. canary has failed), the StatefulSet is annotated as being stale.

### Detect if StatefulSet versions is running

During upgrades, there is more than one version for an ExtendedStatefulSet resource.

Ability to look at what versions are available, and store versions status that keeps track of if version is running:

```yaml
status:
  versions:
    "1": true
    "2": false

```

One version is running is mean that at least one pod that belongs to this StatefulSet is running.

When latest version is running, any version smaller than the greatest version running is deleted.
```yaml
status:
  versions:
    # version 1 was cleaned up
    "2": true 

```

Controller will continue to reconcile until there's only one version.

## Example Resource

```yaml
---
apiVersion: fissile.cloudfoundry.org/v1alpha1
kind: ExtendedStatefulSet
metadata:
  name: MyExtendedStatefulSet
spec:
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
