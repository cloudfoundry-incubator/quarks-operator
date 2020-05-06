# Service wait for Kubernetes native pods

To support clean deployments and correct depedency management, the quarks-operator allows a Kubernetes `Pod` to wait until one (or more) `Service` is available.

The operator does that by injecting an `InitContainer` which waits for the service to be up.

The `Pod`s needs to have the `quarks.cloudfoundry.org/wait-for` annotation, for example:

```yaml
apiVersion: v1
 kind: Pod
 metadata:
   annotations:
     quarks.cloudfoundry.org/wait-for: '[ "nats-headless" , "nginx" ]'
```