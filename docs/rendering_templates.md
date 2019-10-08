# Rendering BOSH Templates

- [Rendering BOSH Templates](#rendering-bosh-templates)
  - [Flow](#flow)
    - [Data Gathering](#data-gathering)
      - [Extract Job Spec and Templates from Image](#extract-job-spec-and-templates-from-image)
      - [Calculation of Required Properties for an Instance Group and BPM Info](#calculation-of-required-properties-for-an-instance-group-and-bpm-info)
    - [Run](#run)
      - [Create ExtendedStatefulSet and ExtendedJobs](#create-extendedstatefulset-and-extendedjobs)
      - [Render Templates](#render-templates)
      - [Run the entrypoints](#run-the-entrypoints)
  - [Details](#details)
    - [Services and DNS Addresses](#services-and-dns-addresses)
    - [Resolving Links](#resolving-links)
    - [Calculating spec.* and link().instances[].*](#calculating-spec-and-linkinstances)
  - [FAQ](#faq)

You can read more about BOSH templates on [bosh.io](https://bosh.io/docs/jobs/#templates).

## Flow

![rendering-flow](https://docs.google.com/drawings/d/e/2PACX-1vRhPrJuMLVXNuFsym_BZdR_RCpknl1eEBwmECLmP8EJLhI4M1HISBbgfA9rfATeEgZW3hwZMPcWxjCI/pub?w=1749&h=1867)

The following points describe each process that involves working with BOSH Job Templates, from beginning to end.

### Data Gathering

The Data Gathering step is run using one `ExtendedJob`, that has one pod with multiple containers.

#### Extract Job Spec and Templates from Image

This happens in one init container for each release present in the deployment manifest.

The entrypoint of that init container is responsible with copying the contents of `/var/vcap/jobs-src` to a shared directory, where other containers can access it.
This shared directory is `/var/vcap/all-releases/jobs-src`.

Each init container uses the release's docker image.

#### Calculation of Required Properties for an Instance Group and BPM Info

The main purpose of the data gathering phase is to compile all information required for all templates to be rendered and for all instance groups to be run:

- properties
- link instances
- bpm yaml

Two containers are run for each instance group in the deployment manifest, using the image of the CF Operator. These two containers write the following on to a file `output.json` in the volume mount `/mnt/quarks` of the container :

- A `Secret` named `<deployment-name>.ig-resolved.<instance-group>-v<version>`

    This is the "Resolved Instance Group Properties" yaml file.
    It contains a deployment manifest structure that only has information pertinent to an instance group.
    It includes:

    - all job properties for that instance group
    - all properties for all jobs that are link providers to any of the jobs of that instance group
    - the rendered contents of each `bpm.yml.erb`, for each job in the instance group
    - link instance specs for all AZs and replicas; read more about instance keys available for links [here](https://bosh.io/docs/links/#templates)

    > **Note:**
    >
    > Link instance specs are stored in the `quarks` property key for each job in the instance group.

- a `Secret` named `<deployment-name>.bpm.<instance-group>-v<version>`

    Once all properties and link instances are compiled, `bpm.yml.erb` can be rendered for each job and for each AZ and replica of the instance group.

    The output of this container is the "BPM Info" yaml file.
    It contains a deployment manifest structure that only has information pertinent to an instance group.
    It includes the rendered contents of each `bpm.yml.erb`, for each job in the instance group.

    > **Note:**
    >
    > The BPM information is stored under the `quarks` property, for each BOSH Job.
    >
    > **Important:**
    >
    > Because container entrypoints in Kubernetes cannot be different among the replicas of a Pod, we don't support the usage of things like `spec.index` in the ERB template of `bpm.yaml`.

### Run

#### Create ExtendedStatefulSet and ExtendedJobs

The operator creates definitions for `ExtendedStatefulSets` (for **BOSH Services**) or `ExtendedJobs` (for **BOSH Errands**).

These have the following init containers:

- one for each unique release in the instance group - used for copying release job specs and templates; these use the release image

- one init container that performs ERB rendering; this runs using the CF Operator image

#### Render Templates

Init containers copy the templates of the releases to `/var/vcap/all-releases`, which is a shared directory among all containers.

Another init container is run using the operator's image, for rendering all templates. It mounts the "Resolved Instance Group Properties" `Secret` (generated in the [data gathering step](#data-gathering)) and performs ERB rendering.
It's also configured with the following environment variables, to facilitate BOSH `spec.*` property keys:

- `INSTANCE_GROUP_NAME`
- `AZ_INDEX`
- `REPLICAS`

#### Run the entrypoints

Once all the init containers are done, all control scripts and configuration files are available on disk, the BOSH Job containers can start.
Their entrypoints, env vars, capabilities, etc. are set based on [BPM information](https://bosh.io/docs/bpm/config/#process-schema).

## Details

The following section describes specific implementation details for algorithms required in the rendering process.

### Services and DNS Addresses

DNS Addresses for instance groups are calculated in the following manner:

```text
<DEPLOYMENT_NAME>-<INSTANCE_GROUP_NAME>-<INDEX>.<KUBE_NAMESPACE>.<KUBE_SERVICE_DOMAIN>
```

The `INDEX` is calculated using the following formula:

```text
(AZ_INDEX - 1) * REPLICAS + POD_ORDINAL
```

In order for things to work correctly across versions and AZs, we need [ClusterIP `Services`](https://kubernetes.io/docs/tutorials/stateful-application/basic-stateful-set/#using-stable-network-identities) that select for Instance Group `Pods`.

For example, assuming a `REPLICAS` of `3` and an `AZ_COUNT` of `2` for a "nats" `BOSHDeployment`, with `2` `StatefulSet` versions available, we would see the following `Services`:

```text
nats-deployment-nats-0
  selects pod z0-v1-0
  selects pod z0-v2-0
nats-deployment-nats-1
  selects pod z1-v1-0
  selects pod z1-v2-0
nats-deployment-nats-2
  selects pod z0-v1-1
  selects pod z0-v2-1
nats-deployment-nats-3
  selects pod z1-v1-1
  selects pod z1-v2-1
nats-deployment-nats-4
  selects pod z0-v1-2
  selects pod z0-v2-2
nats-deployment-nats-5
  selects pod z1-v1-2
  selects pod z1-v2-2
```

### Resolving Links

The following steps describe how to resolve links assuming all information is available. The actual implementation transforms data and stores it in between steps, but the outcome is the same.

To resolve a link, the following steps are performed:

> Vocabulary:
>
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
  Without that, we don't have an entrypoint, env vars, etc. - so we can't create a pod and containers for the BOSH Job.

- Why run all release images for **Data Gathering**?

  We need to run everything all at once because of links. The only way to resolve them is to have all the BOSH Job specs available in one spot.

- Is everything supported in templates, just like BOSH?

  It should, yes. All features should work the same (that's the goal).

  > **Known Exceptions:**
  >
  > - The use of `spec.ip` in `bpm.yml.erb`
  >
  >   Since `bpm.yml` is rendered before the actual instance group runs, in a different pod, `spec.ip` is invalid.
  >
  > - The use of `spec.index` in `bpm.yml.erb`
  >
  >   Any BPM information that is different for each replica, cannot be supported by the CF Operator, because all `Pod` replicas are identical by definition.
