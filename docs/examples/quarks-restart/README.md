## Use Cases

- [Use Cases](#use-cases)
  - [secret.yaml](#secretyaml)
  - [deployment.yaml](#deploymentyaml)
  - [statefulset.yaml](#statefulsetyaml)

### secret.yaml

This is the `Secret` which has the quarks restart-on-update annotation.

### deployment.yaml

This is the `Deployment` which refers to the `Secret`. Whenever the secret's data is modified, the `Deployment` is restarted.

### statefulset.yaml

This is the `StatefulSet` which refers to the `Secret`. Whenever the secret's data is modified, the `StatefulSet` is restarted.
