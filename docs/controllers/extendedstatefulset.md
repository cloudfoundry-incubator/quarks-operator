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

`ExtendedStatefulSets` and `StatefulSets` have a `fissile.cloudfoundry.org/finalizer` `Finalizer`. This allows the operator to perform additional cleanup logic, which prevents owned `ConfigMaps` and `Secrets` from being deleted.

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
<<<<<<< HEAD
<<<<<<< HEAD
The operator can list available versions and store status that keeps track of which is running:
=======
The ability to list available versions, and store versions status that keeps track of whether a version is running:
>>>>>>> Add webhook scaffolding to the operator
=======
The ability to list available versions, and store versions status that keeps track of whether a version is running:
>>>>>>> a028cb472ee656cbc66f581a8a279dcd4a458f61

```yaml
status:
  versions:
    "1": true
    "2": false

```

<<<<<<< HEAD
<<<<<<< HEAD
A version running means that at least one pod that belongs to a `StatefulSet` is running.
When a version **n** is running, any version lower than **n** is deleted.
=======
A version running means that at least one pod that belongs to the `StatefulSet` is running.
When a version is running, any version lower than it is deleted.
>>>>>>> Add webhook scaffolding to the operator
=======
A version running means that at least one pod that belongs to the `StatefulSet` is running.
When a version is running, any version lower than it is deleted.
>>>>>>> a028cb472ee656cbc66f581a8a279dcd4a458f61

```yaml
status:
  versions:
    # version 1 was cleaned up
    "2": true
<<<<<<< HEAD
<<<<<<< HEAD
```

The controller continues to reconcile until there's only one version.

### Volume Management

![Volume Claim management across versions](https://docs.google.com/drawings/d/e/2PACX-1vSvQkXe3zZhJYbkVX01mxS4PKa1iQmWyIgdZh1VKtTS1XW1lC14d1_FHLWn2oA7GVgzJCcEorNVXkK_/pub?w=1185&h=1203)

### AZ Support

The `zones` key defines the availability zones the `ExtendedStatefulSet` needs to span.

The `zoneNodeLabel` defines the node label that defines a node's zone.
The default value for `zoneNodeLabel` is `failure-domain.beta.kubernetes.io/zone`.

The example below defines an `ExtendedStatefulSet` that should be deployed in two availability zones, **us-central1-a** and **us-central1-b**.

```yaml
apiVersion: fissile.cloudfoundry.org/v1alpha1
kind: ExtendedStatefulSet
metadata:
  name: MyExtendedStatefulSet
spec:
  zoneNodeLabel: "failure-domain.beta.kubernetes.io/zone"
  zones: ["us-central1-a", "us-central1-b"]
  ...
  template:
    spec:
      replicas: 2
  ...
=======
>>>>>>> a028cb472ee656cbc66f581a8a279dcd4a458f61
```

The `ExtendedStatefulSet` controller creates one `StatefulSet` version for each availability zone, and adds affinity information to the pods of those `StatefulSets`:

```yaml
affinity:
  nodeAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
      nodeSelectorTerms:
      - matchExpressions:
        - key: "failure-domain.beta.kubernetes.io/zone"
          operator: In
          values: ["us-central1-a"]
=======
>>>>>>> Add webhook scaffolding to the operator
```

If zones are set for an `ExtendedStatefulSet`, the following occurs:

- The name of each created `StatefulSet` is generated as `<extended statefulset name>-z<index of az>-v<statefulset version>`.

  ```text
  myextendedstatefulset-z0-v1
  ```

- The `StatefulSet` and its `Pods` are labeled with the following:

  ```yaml
  fissile.cloudfoundry.org/az-index: "0"
  fissile.cloudfoundry.org/az-name: "us-central1-a"
  ```

- The `StatefulSet` and its `Pods` are annotated with an **ordered** JSON array of all the availability zones:

  ```yaml
  fissile.cloudfoundry.org/zones: '["us-central1-a", "us-central1-b"]'
  ```

- As defined above, each pod is modified to contain affinity rules.
- Each container and init container of each pod have the following env vars set:

  ```shell
  KUBE_AZ="zone name"
  BOSH_AZ="zone name"
  CF_OPERATOR_AZ="zone name"
  CF_OPERATOR_AZ_INDEX=="zone index"
  ```

### Volume Management

### AZ Support

TODO

### Volume Management

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
<<<<<<< HEAD
<<<<<<< HEAD
  # Name of the label that defines the zone for a node
  zoneNodeLabel: "failure-domain.beta.kubernetes.io/zone"
  # List of zones this ExtendedStatefulSet should be deployed on
  zones: ["us-central1-a", "us-central1-b"]
=======
  az:
    
>>>>>>> Add webhook scaffolding to the operator
=======
  az:
    
>>>>>>> a028cb472ee656cbc66f581a8a279dcd4a458f61
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
    spec:
      replicas: 2
      selector:
        matchLabels:
          app: "myapp"
      template:
        metadata:
          labels:
            app: "myapp"
        spec:
          containers:
          - name: "busybox"
            image: "busybox:latest"
            command:
            - "sleep"
            - "3600"

```
