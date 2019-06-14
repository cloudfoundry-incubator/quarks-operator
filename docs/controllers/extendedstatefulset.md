# ExtendedStatefulSet

- [ExtendedStatefulSet](#extendedstatefulset)
  - [Description](#description)
  - [Features](#features)
    - [Scaling Restrictions (not implemented)](#scaling-restrictions-not-implemented)
    - [Automatic Restart of Containers](#automatic-restart-of-containers)
    - [Extended Upgrade Support](#extended-upgrade-support)
    - [Detects if StatefulSet versions are running](#detects-if-statefulset-versions-are-running)
    - [Volume Management](#volume-management)
    - [AZ Support](#az-support)
  - [`ExtendedStatefulSet` Examples](#extendedstatefulset-examples)

## Description

## Features

### Scaling Restrictions (not implemented)

Ability to set restrictions on how scaling can occur: min, max, odd replicas.

### Automatic Restart of Containers

When an env value or mount changes due to a `ConfigMap` or `Secret` change, containers are restarted.
The operator watches all the `ConfigMaps` and `Secrets` referenced by the `StatefulSet`, and automatically performs the update, without extra workarounds.

### Extended Upgrade Support

When an update needs to happen, a second `StatefulSet` for the new version is deployed, and both coexist until canary conditions are met. This also allows support for Blue/Green techniques.

> **Note:**
>
> This could make integration with [Istio](https://istio.io/) easier and (more) seamless.

Annotated with a version (auto-incremented on each update). The annotation key is `fissile.cloudfoundry.org/version`.

Ability to upgrade even though `StatefulSet` pods are not ready.

### Detects if StatefulSet versions are running

During upgrades, there is more than one `StatefulSet` version for an `ExtendedStatefulSet` resource. The operator lists available versions and keeps track of which are running.

A running version means that at least one pod that belongs to a `StatefulSet` is running. When a version **n** is running, any version lower than **n** is deleted.

The controller continues to reconcile until there's only one version.

### Volume Management

The problem we're solving here is the following:

When we create an `ExtendedStatefulSet`, the version associated to it is **v1**. After an update, the `ExtendedStatefulSet` moves on to **v2** with a Blue/Green update strategy. The task is to replace the new `PersistentVolumeClaims` from the **v2** `StatefulSet` with the `PVCs` of **v1**. This is not something that the `StatefulSet` controller supports - it's always trying to recreate the replaced `PVCs` and reattach them to pods.

Our solution is to use a "dummy" `StatefulSet`(with the prefix "volume-management-") with the same replica count as the `ExtendedStatefulSet` replica count. We then wait for this "dummy" `StatefulSet` to generate the volumes that we need.
The final step is to remove the `volumeClaimTemplates` from the actual "desired" `StatefulSets` and mutate the pods so they use the volumes from the "dummy" `StatefulSet`.

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
  AZ_INDEX=="zone index"
  ```
  
## `ExtendedStatefulSet` Examples

See https://github.com/cloudfoundry-incubator/cf-operator/tree/master/docs/examples/extended-statefulset
