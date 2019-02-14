# Rendering BOSH Templates

- [Rendering BOSH Templates](#rendering-bosh-templates)
  - [Part 1 - Data Gathering](#part-1---data-gathering)
  - [Part 2 - Rendering](#part-2---rendering)
    - [Addresses](#addresses)
    - [Input & Output](#input--output)
    - [Properties & Links](#properties--links)
      - [Resolving Links](#resolving-links)

You can read more about BOSH templates on [bosh.io](https://bosh.io/docs/jobs/#templates).

Rendering happens using `ExtendedJobs` and **Init Containers**.

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

## Part 1 - Data Gathering

The first step is data gathering.
An `ExtendedJob` is created for each release.
Each `ExtendedJob` runs one container for each `bosh job` referenced in the `desired manifest`.
Each container outputs a base64 encoded tar gzip fo the entire job folder that it's responsible for.

```yaml
---
apiVersion: fissile.cloudfoundry.org/v1alpha1
kind: ExtendedJob
metadata:
  name: "<DEPLOYMENT_NAME>-<RELEASE_NAME>-spec-reader"
spec:
  run: "now"
  output:
    namePrefix: "<DEPLOYMENT_NAME>-<RELEASE_NAME>-"
    writeOnFailure: false
    outputType: "json"
    secretLabels:
      deployment: '<DEPLOYMENT_NAME>'
      bosh-release: 'release-a'
      bosh-job-name: 'job1'
    updateOnConfigChange: true
  template:
    spec:
      restartPolicy: 'OnFailure'
      containers:
      - name: 'job1'
        image: 'cfcontainerization/release-a:opensuse-42.3-26.gfed099b-30.70-1.76.0'
        command: ['bash', '-c', 'echo -n "{\"spec\":\"$(tar -zcf - -C "/var/vcap/jobs-src/$JOB_NAME/" . | base64 --wrap=0)\"}"']
        env:
        - name: 'JOB_NAME'
          value: 'job1'
```

The command used in the example below can be used for any BOSH release and job.

```shell
bash -c echo -n "{\"spec\":\"$(tar -zcf - -C "/var/vcap/jobs-src/$JOB_NAME/" . | base64 --wrap=0)\"}"
```

This results in multiple **BOSH Job Spec Secrets** being created, each labeled with deployment, release and job identifiers.

## Part 2 - Rendering

Given the information in the **BOSH Job Spec Secrets** and the **Desired Manifest**, the operator can gather all required input information:

- a list of all **BOSH Releases**, **BOSH Jobs** and **BOSH Job Templates** that require rendering
- a list of all **BOSH Variables** and their corresponding `ExtendedSecrets`
- all **BOSH Properties** - both from specs and the **Desired Manifest**
- an understanding of what type of template is being rendered (binary or not)
- the number of replicas for each instance group
- the number of availability zones for each instance group

### Addresses

Addresses for individual pods are calculated in the following manner:

```
<INSTANCE_GROUP_NAME>-<INDEX>-<DEPLOYMENT_NAME>.<KUBE_NAMESPACE>.<KUBE_SERVICE_DOMAIN>
```

> E.g.: `api-group-0-cfdeployment.mycf.svc.cluster.local`

The index is calculated as the index of the pod, multiplied by the AZ index of the pod.
A **Kubernetes Service** is created for each pod of a StatefulSet, that has a selector for the correct pod name(s).
e.g.

```yaml
apiVersion: v1
kind: Service
metadata:
  name: api-group-0-cfdeployment
spec:
  type: ClusterIP
  selector:
    statefulset.kubernetes.io/pod-name: api-group-v1-0
    statefulset.kubernetes.io/pod-name: api-group-v2-0
  ports:
  - protocol: TCP
    port: 80
    targetPort: 80
```

Service names can only consist of lowercase alphanumeric characters, and the character `"-"`.
All `"_"` characters are replaced with `"-"`. All other non-alphanumeric characters are removed. 

The `servicename` cannot start or end with a `"-"`. These characters are trimmed.

Service names are also restricted to 63 characters in length, so if a generated name exceeds 63 characters, it should be recalculated as:

```
servicename=<INSTANCE_GROUP_NAME>-<INDEX><DEPLOYMENT_NAME>

<servicename trimmed to 31 characters><md5 hash of servicename>
```

The same check needs to apply to the entire address. If an entire address is longer than 253 characters, the `servicename` is trimmed until there's enough room for the MD5 hash. If it's not possible to include the hash (`KUBE_NAMESPACE` and `KUBE_SERVICE_DOMAIN` and the dots are 221 characters or more), an error is thrown.

### Input & Output

Rendering happens using the Operator's docker image.
One rendering `ExtendedJob` is run for each **BOSH Job** in each **Instance Group** in the **Desired Manifest**.

The input to an actual rendering container for the templates of a **Instance Group** (not data gathering) is:

- the **Desired Manifest** in the form of a `Secret` mount to `/var/vcap/rendering/manifest.yaml`
-  the generated `Secrets` for all **BOSH Variables** used in the **Instance Group** or, used in any of the **BOSH Jobs** consumed by it (via **BOSH Links**),  each as a mount in `/var/vcap/rendering/variables/<SECRET_NAME>`

We take advantage of the `ExtendedJob`'s feature to persist output in a `Secret`.

Example `ExtendedJob`:

```yaml
---
apiVersion: fissile.cloudfoundry.org/v1alpha1
kind: ExtendedJob
metadata:
  name: "<DEPLOYMENT_NAME>-<RELEASE_NAME>-<JOB_NAME>-renderer"
spec:
  run: "now"
  output:
    namePrefix: "<DEPLOYMENT_NAME>-<RELEASE_NAME>-<JOB_NAME>"
    writeOnFailure: false
    outputType: "json"
    secretLabels:
      bosh-deployment: '<DEPLOYMENT_NAME>'
      bosh-release: 'release-a'
      bosh-job-name: 'job1'
    updateOnConfigChange: true
  template:
    spec:
      restartPolicy: 'OnFailure'
      containers:
      - name: '<HASH_OF_INSTANCE_GROUP_AND_TEMPLATE_NAME>'
        image: 'cfcontainerization/operator:1.0.0'
        command: ['/bin/render-everything']
        env:
        - name: 'JOB_NAME'
          value: 'job1'
        - name: 'TEMPLATE'
          value: 'bin/my_ctl.erb'
```

The output of the rendering process is a JSON object that contains the name of the template and the index of the replica as a key, and the rendered template as a value.

The following is an example output for `instance-group-2` and `release-a`
```json
{
    "start_ctl.sh-0": "#!/bin/sh\necho hello from replica 0",
    "start_ctl.sh-1": "#!/bin/sh\necho hello from replica 1"
}
```

All replicas of a rendered template are mounted. The **init container** that runs in the pod of an **Instance Group** is responsible with symlinking the correct template index to its final location, where they can be used.

We perform rendering in a distinct step **before** running, because of BPM. We require all the rendered information _before_ running. A good example is the entrypoint for a container. With BPM, the entrypoint for replica 0 can be different from replica 1. 

### Properties & Links 

**BOSH Properties** are used straight from the mounted **Desired Manifest Subset**.

**BOSH Properties** that reference **BOSH Variables** have their values set at the time of rendering using the mounted `Secrets`.

All **BOSH Links** are resolved at the time of rendering.

#### Resolving Links

To resolve a link, the following steps are performed:

    > Vocabulary:
    > - `current job` - the job for which rendering is happening
    > - `desired manifest` - the deployment manifest used
    > - `provider job` - the job that has been identified to be the provider for a link

  - the name and type of the link is retrieved from the spec of the `current job`
  - the name of the link is looked up in the `current job`'s instance group `consumes` key (an explicit link definition); if found and is set to `nil`, nil is returned and resolving is complete
  - if the link's name has been overridden by an explicit link definition in the `desired manifest`, the `desired manifest` is searched for a corresponding job, that has the same name; if found, the link is populated with the properties of the `provider job`; first, the defaults for the exposed properties (defined in the `provides` section of the spec of the `provider job`) are set to the defaults from the spec, and then the properties from the `desired manifest` are applied on top
  - if there was no explicit override, we search for a job in all the releases, that provides a link with the same `type` 

The `spec` of each job instance can be calculated by the rendering process in each `ExtendedJob`. The required input is the count of replicas for each instance group, and the AZ count, both available in the deployment manifest.

```yaml
name: <name of the job> 
# TODO: understand if indexes repeat across AZs in BOSH
index: <index of pod>*<az index>
az: <BOSH_AZ_INDEX>
id: <name of the job>-<index>
address: <calculated address>
bootstrap: <index == 0>
```

  > Note: the `deployment`, `network` and `ip_addresses` keys are not supported by the CF Operator

  > Read more about links here: https://bosh.io/docs/links
