# Desired Manifests

A desired manifest is a BOSH deployment manifest that has already been rendered (so it's the actual final state that the user wishes the cluster to be in).

This manifest needs to be persisted and versioned.

Each manifest version that goes live must is stored and is immutable.
A manifest's version is an integer that gets incremented.
The _current version_ of the manifest is the greatest version.


Manifest versions are kept in a secret named `<operator-namespace>/deployment-<deployment-name>-<version>`.
Each secret is also annotated and labeled with information such as:
- the deployment name
- its version
- a description of the "sources" used to render the manifest (e.g. the location of the CRD that generated it). 
