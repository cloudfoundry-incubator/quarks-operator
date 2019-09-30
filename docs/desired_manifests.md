# Desired Manifests

- [Desired Manifests](#desired-manifests)

A desired manifest is a BOSH deployment manifest that has already been calculated so that it's the actual final state that the user wishes his software to be in. All ops files have been applied, variables have been calculated and replaced. This manifest is persisted and versioned.

Ops files are applied by the operator.
Variables are replaced by an `ExtendedJob` that runs the operator's image. The `ExtendedJob` writes the manifest on stdout, which is persisted using a [Versioned Secret](controllers/extendedjob.md#versioned-secrets).

Each manifest version that goes live is immutable.
A manifest's version is an integer that gets incremented.
The _current version_ of the manifest is the greatest version.

These manifests are kept in secrets named using the following rule:

```plain
<operator-namespace>/<deployment-name>.desired-manifest-v<version>
```

- `deployment-name`: the name of deployment manifest
- `version`: the version of manifest

Each secret is also annotated and labeled with information such as:

- the deployment name
- the secret kind
- its version
- a description of the "sources" used to render the manifest (e.g. the location of the CRD that generated it).
