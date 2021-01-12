# Quarks-operator

[![godoc](https://godoc.org/code.cloudfoundry.org/quarks-operator?status.svg)](https://godoc.org/code.cloudfoundry.org/quarks-operator)
[![master](https://ci.flintstone.cf.cloud.ibm.com/api/v1/teams/quarks/pipelines/cf-operator/badge)](https://ci.flintstone.cf.cloud.ibm.com/teams/quarks/pipelines/cf-operator)
[![go report card](https://goreportcard.com/badge/code.cloudfoundry.org/quarks-operator)](https://goreportcard.com/report/code.cloudfoundry.org/quarks-operator)
[![Coveralls github](https://img.shields.io/coveralls/github/cloudfoundry-incubator/quarks-operator.svg?style=flat)](https://coveralls.io/github/cloudfoundry-incubator/quarks-operator?branch=HEAD)

| Nightly build | [![quarks-operator-nightly](https://github.com/cloudfoundry-incubator/quarks-operator/workflows/quarks-operator-ci/badge.svg?event=schedule)](https://github.com/cloudfoundry-incubator/quarks-operator/actions?query=event%3Aschedule) |
| ------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |

<img align="right" width="200" height="39" src="https://quarks.suse.dev/docs/cf-operator-logo.png">

----

Quarks-operator enables the deployment of BOSH Releases, especially Cloud Foundry, to Kubernetes.

It's implemented as a k8s operator, an active controller component which acts upon custom k8s resources.

----


* Incubation Proposal: [Containerizing Cloud Foundry](https://docs.google.com/document/d/1_IvFf-cCR4_Hxg-L7Z_R51EKhZfBqlprrs5NgC2iO2w/edit#heading=h.lybtsdyh8res)
* Slack: #quarks-dev on <https://slack.cloudfoundry.org>
* Backlog: [Pivotal Tracker](https://www.pivotaltracker.com/n/projects/2192232)
* Helm: https://hub.helm.sh/charts/quarks/quarks
* Docker: https://github.com/orgs/cloudfoundry-incubator/packages/container/package/quarks-operator
* Documentation: [quarks.suse.dev](https://quarks.suse.dev)

----

- [Features](https://quarks.suse.dev/docs/quarks-operator/overview/)
   - [Controllers](https://quarks.suse.dev/docs/quarks-operator/development/controllers/)
   - [BOSH Variables interpolation](https://quarks.suse.dev/docs/quarks-operator/concepts/variables/)
- [Installing](https://quarks.suse.dev/docs/quarks-operator/install/)
  - [Troubleshooting](https://quarks.suse.dev/docs/quarks-operator/troubleshooting/)
- [Tooling](https://quarks.suse.dev/docs/development/tooling/)

## Development and Tests

Also see [CONTRIBUTING.md](CONTRIBUTING.md).

For more information about

* the operator development, see [development docs](https://quarks.suse.dev/docs/development/)
* testing, see [testing docs](https://quarks.suse.dev/docs/development/testing/)
* building the operator from source, see [here](https://quarks.suse.dev/docs/development/building/)
* how to develop a BOSH release using Quarks and SCF, see the [SCFv3 docs](https://github.com/SUSE/scf/blob/v3-develop/dev/scf/docs/bosh-author.md)


## Quarks Meta

* https://github.com/cloudfoundry-incubator/quarks-job - helm chart to run jobs and persist their output to secrets
* https://github.com/cloudfoundry-incubator/quarks-secret - helm chart to generate credentials in cluster
* https://github.com/cloudfoundry-incubator/quarks-statefulset - helm chart to improve on statefulsets
* https://github.com/cloudfoundry-incubator/quarks-utils - library with shared functionality
* https://github.com/cloudfoundry-incubator/quarks-container-run - command wrapper, which supports a conditional 'post start' command
* https://github.com/cloudfoundry-incubator/quarks-docs - the sources for https://quarks.suse.dev/
* https://github.com/cloudfoundry-incubator/quarks-gora - a simple server, used in testing
* https://github.com/cloudfoundry-incubator/quarks-gora-release - a BOSH release for quarks-gora, used in testing
* https://github.com/cloudfoundry-incubator/quarks-helm - the helm repo for all Quarks charts
* https://github.com/cloudfoundry-incubator/quarks-ci - CI pipelines, also contains the [fissile pipeline](https://github.com/cloudfoundry-incubator/quarks-ci/tree/master/pipelines/release-images-cf-deployment) to create the BOSH release images
* https://github.com/cloudfoundry-incubator/fissile - command line tool to convert BOSH releases into Docker images
* https://github.com/viovanov/bosh-template-go - a Go wrapper for the bosh-template gem, to render job specs
