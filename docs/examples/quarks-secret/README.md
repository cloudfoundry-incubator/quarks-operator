## Use Cases

- [Use Cases](#use-cases)
  - [password.yaml](#passwordyaml)
  - [rotate.yaml](#rotateyaml)
  - [copies.yaml and copy-secret-destination.yaml](#copiesyaml-and-copy-secret-destinationyaml)

### password.yaml

This generates a password in a Kubernetes `Secret`.

### rotate.yaml

This is a rotation config, which will re-generate the password from password.yaml

### copies.yaml and copy-secret-destination.yaml

These two files show how you could generate a secret value, and have it shared in multiple namespaces
