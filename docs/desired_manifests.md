# Desired Manifests

- [Desired Manifests](#desired-manifests)
  - [Flow](#flow)
  - [Naming Conventions](#naming-conventions)

A desired manifest is a BOSH deployment manifest that has already been rendered (so it's the actual final state that the user wishes the cluster to be in). All ops files have been applied and variables have been calculated and replaced. This manifest is persisted and versioned.

Ops files are applied by the operator.
Variables are replaced by a `Job` that runs the operator's image. This is the process that calculates a version for the Desired Manifest and persists it.

Each manifest version that goes live is stored and is immutable.
A manifest's version is an integer that gets incremented.
The _current version_ of the manifest is the greatest version.

Manifest versions are kept in a secret named `<operator-namespace>/deployment-<deployment-name>-<version>`.
Each secret is also annotated and labeled with information such as:

- the deployment name
- its version
- a description of the "sources" used to render the manifest (e.g. the location of the CRD that generated it).

## Flow

![flow](https://docs.google.com/drawings/d/e/2PACX-1vSsapirEQTlBvFDYjRbCxK5IJaxRqPDfTi37OcBVr60BGbDThn83HeHJyRModicUeghc7enfyYY_CHI/pub?w=1279&h=997)

## Naming Conventions

> TODO: complete this section

- `Secrets` for "manual" variables
- `ExtendedSecrets` for generated variables
- `Job` for variable replacement
- Temp Manifest `Secret`
- Desired Manifest `Secret`
