# ExtendedJob

- [ExtendedJob](#extendedjob)
  - [Description](#description)
  - [Features](#features)
    - [Triggered Jobs](#triggered-jobs)
      - [State](#state)
      - [Labels](#labels)
    - [One-Off Jobs / Auto-Errands](#one-off-jobs-auto-errands)
    - [Errand Jobs](#errand-jobs)
    - [Persisted Output](#persisted-output)
  - [Example Resource](#example-resource)

## Description

An `ExtendedJob` allows the developer to run jobs when something interesting happens. It also allows the developer to store the output of the job into a `ConfigMap` or `Secret`.

Just like an `ExtendedStatefulSet`, an `ExtendedJob` can automatically be restarted if its environment/mounts have changed due to a `ConfigMap` or a `Secret` being updated.

There are three different kinds of `ExtendedJob`: triggered jobs, one-offs and
errands.

## Features

### Triggered Jobs

An `ExtendedJob` can be triggered when something interesting happens to a pod.

E.g. when a `Pod` is created, deleted, transitioned to **ready** or a
**notReady** state.

The execution of `ExtendedJob` can be limited to pods with certain labels.

A separate native k8s job will be started for every pod that changed. The job
has a label `affected-pod: pod1` to identify which pod it is running for.

#### State

To trigger on the state of the pod, the `when` trigger can be used.
Possible values are `ready`, `notready`, `created` and `deleted`.

The `when` field is required for triggered jobs.

Example: `when: ready`

#### Labels

To trigger on pods with a matching label, the `selector` trigger can be used.
It supports matching against a list of labels via `matchLabels`.
It can also match by expressions if `matchExpressions` are given.

If multiple selectors are given, all must match to include the pod.

### One-Off Jobs / Auto-Errands

One-off jobs run directly when created, just like native k8s jobs, but still
persist their output.

They can't have a `triggers` section and specify `run: once` instead.

### Errand Jobs

Errands are manually run by the user.

They can't have a `triggers` section and specify `run: manually` instead.

After the `ExtendedJob` is created, run an errand by editing and applying the
manifest, i.e. via `k edit errand1` and change `run: manually` to `run: now`,
after completion the value will be reset to `manually`.

### Persisted Output

The developer can specify a ConfigMap or a Secret where the standard output/error output of the ExtendedJob is stored.

Since a Job can run multiple times until it succeeds, the behavior of storing the output is controlled by specifying the following parameters:
- `overwrite` - if true, the ConfigMap or Secret is updated on every run
- `writeOnFailure` - if true, output is written even though the Job failed.

## Example Triggered ExtendedJob Resource

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
    when: ready
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
        env:
        - name: MY_ENV
          valueFrom:
            configMapKeyRef:
              name: foo-config
              key: special.how
    backoffLimit: 4
```
