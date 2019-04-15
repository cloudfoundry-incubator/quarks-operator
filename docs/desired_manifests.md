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

Manifest versions are kept in a secret named `<operator-namespace>/<deployment-name>.with-vars.interpolation-v<version>`.

- `deployment-name`: the name of deployment manifest
- `version`: the version of manifest

Each secret is also annotated and labeled with information such as:

- the deployment name
- the secret kind
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

### Manual ("implicit") variables

BOSH deployment manifests support two different types of variables, implicit and explicit ones.

"Explicit" variables are declared in the `variables` section of the manifest and are generated automatically before the interpolation step.

"Implicit" variables just appear in the document within double parentheses without any declaration. These variables have to be provided by the user prior to creating the BOSH deployment. The variables have to be provided as a secret with the `value` key holding the variable content. The secret name has to follow the scheme

```
<deployment-name>.var-implicit-<variable-name>
```

Example:

```
---
apiVersion: v1
kind: Secret
metadata:
  name: nats-deployment.var-implicit-system-domain
type: Opaque
stringData:
  value: example.com
```
