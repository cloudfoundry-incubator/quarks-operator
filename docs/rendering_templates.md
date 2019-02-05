TODO: define service names

# Rendering BOSH Templates

You can read more about BOSH templates on [bosh.io](https://bosh.io/docs/jobs/#templates).

Rendering happens using `ExtendedJobs`.

Details are omitted in the example below, for brevity. This is a deployment manifest.

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

The first step is data gathering.
An `ExtendedJob` is created for each release.
Each `ExtendedJob` runs one container for each `bosh job` referenced in the `desired manifest`.
Each container outputs a base64 encoded tar gzip fo the entire job folder that it's responsible for.

```yaml
---
apiVersion: fissile.cloudfoundry.org/v1alpha1
kind: ExtendedJob
metadata:
  name: "<DEPLOYMENT_NAME>-<RELEASE_NAME>"
spec:
  output:
    stdout:
      prefix: "<DEPLOYMENT_NAME>-<RELEASE_NAME>-"
  template:
    metadata:
    name: "<DEPLOYMENT_NAME>-<RELEASE_NAME>-spec-export"
    spec:
      template:
        spec:
        containers:
        - name: job1
          image: docker.io/cfcontainerization/release-a:opensuse-42.3-26.gfed099b-30.70-1.76.0
          command: []
          
```



  in the with multiple pods, one for each release.
Each pod runs a container for each Job that is referenced in the role manifest.




An `ExtendedJob` is required for each BOSH Release and Instance Group defined in a Desired Manifest.


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


#### Resolving Links

1. Create a job that has a container for each releases
2. All containers have an anv var `RELEASES` listing all available releases
3. All containers copy their `/var/vcap/jobs-src` to `/var/vcap/rendering/<RELEASE_NAME>/*`
4. When done with copying, each container writes `/var/vcap/releases/<RELEASE_NAME>.done`
5. For rendering, the following data structure is created:

```
release
  job
    (contents of spec)
    properties
    consumes
    provides
```

6. To resolve a link, the following steps are performed:

    > Vocabulary:
    > - `current job` - the job for which rendering is happening
    > - `desired manifest` - the deployment manifest used
    > - `provider job` - the job that has been identified to be the provider for a link

  - the name and type of the link is retrieved from the spec of the `current job`
  - the name of the link is looked up in the `current job`'s instance group `consumes` key (an explicit link definition); if found and is set to `nil`, nil is returned and resolving is complete
  - if the link's name has been overridden by an explicit link definition in the `desired manifest`, the `desired manifest` is searched for a corresponding job, that has the same name; if found, the link is populated with the properties of the `provider job`; first, the defaults for for the exposed properties (defined in the `provides` section of the spec of the `provider job`) are set to the defaults from the spec, and then the properties from the `desired manifest` applied on top
  - if there was no explicit override, we search for a job in all the releases, that provides a link with the same `type` 


  > Note: the `deployment`, `network` and `ip_addresses` are not supported by the CF Operator

  > Read more about links here: https://bosh.io/docs/links
