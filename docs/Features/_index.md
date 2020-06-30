---
title: "Features"
linkTitle: "Features"
weight: 11
---

Quarks-operator deploys dockerized BOSH releases onto existing Kubernetes cluster

* Supports operations files to modify manifest
* Service instance groups become pods, each job in one container
* Errand instance groups become QuarksJobs

To do this it relies on three Kubernetes components:

* QuarksSecret, a custom resource and controller for the generation and rotation of secrets
* [QuarksJob](https://github.com/cloudfoundry-incubator/quarks-job), templating for Kubernetes jobs, which can trigger jobs on configuration changes and persist their output to secrets
* QuarksStatefulSet, adds canary, zero-downtime deployment, zones and active-passive probe support
* QuarksRestart, restarts statefulset and deployment if the referenced secret changes

The Quarks-operator supports RBAC and uses immutable, versioned secrets internally.

## Compatibility with BOSH

* Supports BOSH deployment manifests, including links and addons
* Uses available BPM information from job releases
* Renders ERB job templates in an init container, before starting the dockerized BOSH release
* Adds endpoints and services for instance groups
* BOSH DNS support
* Uses Kubernetes zones for BOSH AZs
* Interaction with configuration:
  * BOSH links can be provided by existing Kubernetes secrets
  * Provides BOSH link properties as Kubernetes secrets
  * Generates explicit variables, e.g. password, certificate, and SSH keys
  * Reads implicit variables from secrets
  * Secret rotation for individual secrets

* Adapting releases:
  * Pre-render scripts to patch releases, which are incompatible with Kubernetes

* Lifecycle related:
  * Restart only affected instance groups on update
  * Sequential startup of instance groups
  * Kubernetes healthchecks instead of monit