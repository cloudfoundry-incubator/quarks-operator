# Rendering BOSH Templates

You can read more about BOSH templates on [bosh.io](https://bosh.io/docs/jobs/#templates).

Rendering happens using `ExtendedJobs`.

An `ExtendedJob` is required for each BOSH Release and Instance Group defined in a Desired Manifest.

Details are omitted in the example below, for brevity:

```yaml
---

releases:
- name: release-a
- name: release-b

instance_groups:
- name: instance-group-1
  jobs:
  - name: job1
    release: release-a
  - name: job2
    release: release-a
- name: instance-group-2
  jobs:
  - name: job1
    release: release-a
    properties:
      foo: "((bar))"
  - name: job3
    release: release-b

variables:
- name: bar
  type: password
```

In this case, to render all required templates, 3 `ExtendedJobs` are created:
- one for  `instance-group-1`, because it only contains jobs from `release-a`
- two for `instance-group-2`, because it references jobs from both `release-a` and  `release-b`

The `ExtendedJob` runs a ruby process `TODO: reference to the implementation`

## Input & Output

Input is the desired manifest, generated `Secrets` for all referenced variables and information required for discovery of network addresses.

We take advantage of the `ExtendedJob`'s feature to persist output in a `ConfigMap` or `Secret`.

The output of the rendering process is a JSON object that contains all the rendered templates:

The following is and example definition for the `ExtendedJob` that renders templates for `instance-group-2` and `release-a`.
The release is inferred from the image itself.

```yaml
---
---
apiVersion: fissile.cloudfoundry.org/v1alpha1
kind: ExtendedJob
metadata:
  name: MyExtendedJob
spec:
  output:
    stdout:
      secretRef: "mynamespace/mydeployment-release-a-instance-group-2"
      overwrite: true
      writeOnFailure: false
  updateOnConfigChange: true
  template:
    spec:
      template:
        spec:
        containers:
        - image: "cfcontainerization/capi-release:opensuse-42.3-24.g63783b3-30.66-1.75.0"
          command: ["some-ruby-script-that-renders"]
          env:
          - name: "INSTANCE_GROUP_NAME"
            value: "instance-group-2"
          volumeMounts:
          - name: deployment-manifest
            mountPath: /opt/fissile/
          volumeMounts:
          - name: mydeployment-bar
            mountPath: /opt/fissile/
        volumes:
        - name: deployment-manifest
          secret:
            name: "deployment-mydeployment-12"
            items:
            - key: "manifest"
              path: "deployment-manifest.yaml"
        - name:
          secret:
            name: "mydeployment-bar"
            items:
            - key: "value"
              path: "bar"
```

The following is an example output for `instance-group-2` and `release-a`
```json
{
    "job1":{
        "bin/start_ctl.sh": "#!/bin/sh\necho hello"
    }
}
```

### Properties & Links 

Properties are used straight from the mounted deployment manifest.

Properties that reference variables have their values set at the time of rendering.

Links are resolved at the time of rendering. Because links can reference properties that use variables, we must mount all variables in each of the `ExtendedJobs` that render templates.

> This won't cause superfluous restarts, since `ExtendedStatefulSets` and `ExtendedJobs` are restarted only if the referenced secrets/configmap contents have changed.
