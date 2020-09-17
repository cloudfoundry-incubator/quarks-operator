## Use Cases

- [Use Cases](#use-cases)
  - [secret.yaml](#secretyaml)
  - [deployment.yaml](#deploymentyaml)
  - [statefulset.yaml](#statefulsetyaml)

### secret.yaml

This is the `Secret` being used by the pods, which will trigger restarts on annotated pods.

### deployment.yaml

This is the `Deployment` which refers to the `Secret`. Whenever the secret's data is modified, the `Deployment` is restarted.

### statefulset.yaml

This is the `StatefulSet` which refers to the `Secret`. Whenever the secret's data is modified, the `StatefulSet` is restarted.
