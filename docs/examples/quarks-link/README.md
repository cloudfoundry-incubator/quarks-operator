## Use Cases

- [Use Cases](#use-cases)
  - [boshdeployment-with-external-consumer.yaml](#boshdeployment-with-external-consumeryaml)

### boshdeployment-with-external-consumer.yaml

This is a `BOSHDeployment` which consumes a link of a native Kubernetes `Pod` from a `Deployment`. Before creating the `BOSHDeployment` in the cluster, create the native Kubernetes requirements by creating link-secret, link-pod and link-service resources.
