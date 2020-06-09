---
title: "Service wait for Kubernetes native pods"
linkTitle: "Waiting for services"
weight: 11
---

To support clean deployments and correct depedency management, the quarks-operator allows a Kubernetes `Pod` to wait until one (or more) `Service` is available.

The operator does that by injecting an `InitContainer` which waits for the service to be up.

This is a generalization of the serialization hints natively available to all BOSH deployments.

The `Pod`s needs to have the `quarks.cloudfoundry.org/wait-for` annotation, for example:

```yaml
apiVersion: v1
 kind: Pod
 metadata:
   annotations:
     quarks.cloudfoundry.org/wait-for: '[ "nats-headless" , "nginx" ]'
```

At the ops level this is achieved by an instruction like

```yaml
- type: replace
  path: /instance_groups/name=THE_INSTANCE_GROUP/env?/bosh/agent/settings/annotations/quarks.cloudfoundry.org~1wait-for
  value: '[ "uaa" ]'
```

The `env/bosh/agent/settings/annotations` key is a hash used by the operator to add additional annotations to the k8s objects it creates for an instance group. IOW they are applied to the generated `quarks-statefulset`, `statefulset`, and `pod`.

:warning: Note that while the dependency information is ultimately processed as a json array of strings, at the level of the annotations it has to be specified as a plain string. Just one which contains proper json syntax.

If full custom dependencies are not required, just (partial) serial startup of instance groups (in the order of their specification in their deployment) then the native serialization hints are likely good enough.

They are specified via a construction of the form

```yaml
instance_groups:
- name: THE_INSTANCE_GROUP
  update:
    serial: true
```

in the BOSH deployment, if under direct control, or via an ops file like

```yaml
- type: replace
  path: /instance_groups/name=THE_INSTANCE_GROUP/update/serial
  value: true
```

if the deployment cannot be modified directly.
