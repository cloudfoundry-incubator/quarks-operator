# ExtendedJob

- [ExtendedJob](#extendedjob)
    - [Description](#description)
    - [Features](#features)
        - [Triggers](#triggers)
        - [Persisted Output](#persisted-output)
    - [Example Resource](#example-resource)

## Description

An `ExtendedJob` allows the developer to run jobs when something interesting happens. It also allows the developer to store the output of the job into a `ConfigMap` or `Secret`.

Just like an `ExtendedStatefulSet`, an `ExtendedJob` can automatically be restarted if its environment/mounts have changed due to a `ConfigMap` or a `Secret` being updated. 

## Features

### Triggers

An `ExtendedJob` can be triggered when something interesting happens for an entity.

E.g. when a `Pod` is created, updated, deleted, transitioned to **ready** or a **notReady** state.

The *something* can be a selector.

### Persisted Output

The developer can specify a ConfigMap or a Secret where the standard output/error output of the ExtendedJob is stored.

Since a Job can run multiple times until it succeeds, the behavior of storing the output is controlled by specifying the following parameters:
- `overwrite` - if true, the ConfigMap or Secret is updated on every run
- `writeOnFailure` - if true, output is written even though the Job failed.

## Example Resource

```yaml
---
apiVersion: fissile.cloudfoundry.org/v1alpha1
kind: ExtendedJob
metadata:
  name: MyExtendedJob
spec:
  output:
    stderr:
      configRef: "mynamespace/fooErrors"
      overwrite: true
      writeOnFailure: true
    stdout:
      secretRef: "mynamespace/fooSecret"
      overwrite: true
      writeOnFailure: false
  updateOnConfigChange: true
  triggers:
    when:
      ready: true
    selector:
      matchLabels:
        component: redis
      matchExpressions:
        - {key: tier, operator: In, values: [cache]}
        - {key: environment, operator: NotIn, values: [dev]}

  # Below you can see a template for a regular Job.
  # Nothing else is custom below this point.
  template:
    metadata:
    name: pi
    spec:
    template:
        spec:
        containers:
        - name: pi
            image: perl
            command: ["perl",  "-Mbignum=bpi", "-wle", "print bpi(2000)"]
        restartPolicy: Never
    backoffLimit: 4
```
