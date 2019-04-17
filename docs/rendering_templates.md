# Rendering BOSH Templates

- [Rendering BOSH Templates](#rendering-bosh-templates)
  - [Flow](#flow)
    - [Data Gathering](#data-gathering)
      - [Extract Job Spec and Templates from Image](#extract-job-spec-and-templates-from-image)
      - [Calculation of Required Properties for an Instance Group and Render BPM](#calculation-of-required-properties-for-an-instance-group-and-render-bpm)
    - [Generate Kube Objects](#generate-kube-objects)
      - [Create ExtendedStatefulSet and ExtendedJobs](#create-extendedstatefulset-and-extendedjobs)
    - [Run](#run)
      - [Render Templates](#render-templates)
      - [Run the actual BOSH Jobs](#run-the-actual-bosh-jobs)
  - [Example](#example)
  - [Details](#details)
    - [DNS Addresses](#dns-addresses)
    - [Resolving Links](#resolving-links)
    - [Calculating spec.* and link().instances[].*](#calculating-spec-and-linkinstances)
  - [FAQ](#faq)

You can read more about BOSH templates on [bosh.io](https://bosh.io/docs/jobs/#templates).

## Flow

![rendering-flow](https://docs.google.com/drawings/d/e/2PACX-1vRVULT7NVp932sCdONuhUmOR2wrYdm9Axy_lZAb_FrCT7w-f0hjLnZT4_8uDRCWJ3zZeEBl7rYGryS-/pub?w=1439&h=1684)

The following points describe each process that involves working with BOSH Job Templates, from beginning to end.

### Data Gathering

The Data Gathering step is run using one `ExtendedJob`, that has one pod with multiple containers.

#### Extract Job Spec and Templates from Image

This happens in the "Data Gathering" step, in one init container for each release present in the deployment manifest.

The entrypoint of that init container is responsible with copying the contents of `/var/vcap/jobs-src` to a shared directory, where other containers can access it.

Each init container runs the release's docker image.

#### Calculation of Required Properties for an Instance Group and Render BPM

The main purpose of the data gathering phase is to compile all information required for all templates to be rendered and for all instance groups to be run:

- properties
- link instances
- bpm yaml

One container is run for each instance group in the deployment manifest.
Once all properties and link instances are compiled, `bpm.yml.erb` can be rendered for each job and for each AZ and replica of the instance group.

The output of each container is a deployment manifest structure that only has information pertinent to an instance group.
This includes:

- all job properties for that instance group
- all properties for all jobs that are link providers to any of the jobs of that instance group
- the rendered contents of each `bpm.yml.erb`, for each job in the instance group
- link instance specs for all AZs and replicas; read more about instance keys available for links [here](https://bosh.io/docs/links/#templates)

Both link instance specs as well as the contents of `bpm.yml.erb` are stored in the `bosh_containerization` property key for each job in the instance group.
One BPM object should exist per replica of the instance group, but if they are all the same, we only store one copy. We perform this rendering in a distinct step **before** running because BPM has information we require _before_ running. A good example is the entrypoint for a container.

Each output of this container should be compressed. Size can be large, and secrets are limited to 1MB.

These containers run using the same docker image as the CF Operator.

### Generate Kube Objects

#### Create ExtendedStatefulSet and ExtendedJobs

Operator creates definitions for `ExtendedStatefulSets` and `ExtendedJobs`, for each instance group. They have init containers for all releases in the instance group, as well as an init container that does rendering.

### Run

All instance groups run either as an `ExtendedStatefulSet` (for **BOSH Services**) or as an `ExtendedJobs` (for **BOSH Errands**).

These run pods that have multiple init containers for template rendering.

#### Render Templates

Init containers copy the templates of the releases to `/var/vcap/all-releases`, which is a shared directory among all containers.

Another init container is run using the operator's image, for rendering all templates. It mounts the property secrets for the instance group (generated in the data gathering step) and performs rendering.
It's also configured with the following environment variables, to facilitate BOSH `spec.*` property keys:

- `BOSH_DEPLOYMENT_NAME` - the deployment name
- `BOSH_AZ_INDEX` - current AZ (integer)
- `BOSH_AZ_COUNT` - AZ Count
- `BOSH_REPLICA_COUNT` - replica count
- `BOSH_REPLICA_ORDINAL` - current replica ordinal
- `BOSH_JOB_NAME` - job name

#### Run the actual BOSH Jobs

Once all the init containers are done, all control scripts and configuration files are available on disk, and the BOSH Job containers can start.
Their entrypoints are set based on BPM information.

## Example

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

> TODO: complete example

## Details

The following section describes specific implementation details for algorithms required in the rendering process.

### DNS Addresses

DNS Addresses for instance groups are calculated in the following manner:

```text
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

```text
servicename=<INSTANCE_GROUP_NAME>-<INDEX><DEPLOYMENT_NAME>

<servicename trimmed to 31 characters><md5 hash of servicename>
```

The same check needs to apply to the entire address. If an entire address is longer than 253 characters, the `servicename` is trimmed until there's enough room for the MD5 hash. If it's not possible to include the hash (`KUBE_NAMESPACE` and `KUBE_SERVICE_DOMAIN` and the dots are 221 characters or more), an error is thrown.

### Resolving Links

The following steps describe how to resolve links assuming all information is available. The actual implementation will transform data and store it in between steps, but the outcome must be the same.

To resolve a link, the following steps are performed:

    > Vocabulary:
    > - `current job` - the job for which rendering is happening
    > - `desired manifest` - the deployment manifest used
    > - `provider job` - the job that has been identified to be the provider for a link

- the name and type of the link is retrieved from the spec of the `current job`
- the name of the link is looked up in the `current job`'s instance group `consumes` key (an explicit link definition); if found and is set to `nil`, nil is returned and resolving is complete
- if the link's name has been overridden by an explicit link definition in the `desired manifest`, the `desired manifest` is searched for a corresponding job, that has the same name; if found, the link is populated with the properties of the `provider job`; first, the defaults for the exposed properties (defined in the `provides` section of the spec of the `provider job`) are set to the defaults from the spec, and then the properties from the `desired manifest` are applied on top
- if there was no explicit override, we search for a job in all the releases, that provides a link with the same `type`

  > Read more about links [here](https://bosh.io/docs/links).

### Calculating spec.* and link().instances[].*

The `spec` of each job instance can be calculated:

```yaml
name: <name of the instance group>-<name of the job>
index: (<az index>-1)*<replicas>+<pod_ordinal>
az: <BOSH_AZ_INDEX>
id: <name of the instance group>-<index>-<name of the job>
address: <calculated address>
bootstrap: <index == 0>
```

## FAQ

- Why render BPM separately from all other BOSH Job Templates?

  Because we need information from BPM to actually know what to run.
  Without that, we don't have an entrypoint, env vars, etc. - so we can't create a pod and a containers for the BOSH Job.

- Why run all release images for **Data Gathering**?

  We need to run everything all at once because of links. The only way to resolve them is to have all the BOSH Job specs available in one spot.

- Is everything supported in templates, just like BOSH?

  It should, yes. All feature should work the same (that's the goal).
  There's one exception though - the use of `spec.ip` in `bpm.yml.erb`. Since `bpm.yml` is rendered before the actual instance group runs, in a different pod, `spec.ip` is invalid.
