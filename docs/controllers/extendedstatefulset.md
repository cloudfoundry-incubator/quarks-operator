# ExtendedStatefulSet

- [ExtendedStatefulSet](#extendedstatefulset)
  - [Description](#description)
  - [Features](#features)
    - [Scaling Restrictions](#scaling-restrictions)
    - [Automatic Restart of Containers](#automatic-restart-of-containers)
    - [Extended Upgrade Support](#extended-upgrade-support)
    - [Automatic Management of Roles for the ServiceAccount](#automatic-management-of-roles-for-the-serviceaccount)
    - [Annotated if Stale](#annotated-if-stale)
  - [Example Resource](#example-resource)

## Description

## Features

### Scaling Restrictions

Ability to set restrictions on how scaling can occur: min, max, odd replicas.

### Automatic Restart of Containers

When an env value or mount changes due to a `ConfigMap` or `Secret` change, containers are restarted.
The operator watches all the ConfigMaps and Secrets referenced by the StatefulSet, and automatically performs the update, without extra workarounds.

### Extended Upgrade Support

A second StatefulSet for the new version is deployed, and both coexist until canary conditions are met. This also allows support for Blue/Green tehniques. 

> Note: This could make integration with Istio easier and (more) seamless.

Annotated with a version (auto-incremented on each update). 

Ability to upgrade even though StatefulSet pods are not ready and the ability to run an `ExtendedJob` before and after the upgrade.

The Job can abort the upgrade if it doesn't complete successfully.

### Automatic Management of Roles for the ServiceAccount

### Annotated if Stale

If a failure has occurred (e.g. canary has failed), the StatefulSet is annotated as being stale.

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
  # If true, the StatefulSet will be  
  updateOnEnvChange: true
  updateStrategy:
    canaries: 1
    retryCount: 2
    updateNotReady: true

  # Below you can see a template for a regular StatefulSet.
  # Nothing else is custom below this point.
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