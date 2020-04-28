## Use Cases

- [Use Cases](#use-cases)
  - [secret.yaml](#secretyaml)
  - [deployment.yaml](#deploymentyaml)
  - [deployment.yaml](#deploymentyaml-1)

### secret.yaml

This is the `Secret` which has the quarks update-referenced-owner annotation.

### deployment.yaml

This is a `Deployment` which refers the `Secret`. Whenever the secret's data changes, this `Deployment` gets restarted.

### statefulset.yaml

This is a `StatefulSet` which refers the `Secret`. Whenever the secret's data changes, this `StatefulSet` gets restarted.
