# Entanglements

Also known as "Quarks Links" - they provide a way to share/discover information between BOSH and Kube Native components.

## Native -> BOSH

In this case, the native component is a provider, and the BOSH component is a consumer.

We construct link information like so:

| BOSH Link           | Native  | Description                                                                                              |
| ------------------- | ------- | -------------------------------------------------------------------------------------------------------- |
| address             | Service | DNS address of a Kubernetes service annotated  `quarks.cloudfoundry.org/provides = LINK_NAME`            |
| azs                 | N/A     | not supported                                                                                            |
| p                   |         | properties retrieved from a secret annotated `quarks.cloudfoundry.org/provides = LINK_NAME`              |
| instances.name      | Pod     | name of pod selected by the Kube Service that's annotated `quarks.cloudfoundry.org/provides = LINK_NAME` |
| instances.id        | Pod     | pod uid                                                                                                  |
| instances.index     | Pod     | set to a value 0-(pod replica count)                                                                     |
| instances.az        | N/A     | not supported                                                                                            |
| instances.address   | Pod     | ip of pod                                                                                                |
| instances.bootstrap | Pod     | set to true if index == 0                                                                                |

> If multiple secrets or services are found with the same link information, the operator should error

### Example

```yaml
kind: Secret
metadata:
  annotations:
    quarks.cloudfoundry.org/deployment: "mydeployment"
    quarks.cloudfoundry.org/provides: '{"name":"nats","type":"nats"}'
spec:
  data:
    password: mysecret
```

Using this secret, I should be able to use `link("nats").p("password")` in one of my BOSH templates.

```
apiVersion: v1
kind: Service
metadata:
  annotations:
    quarks.cloudfoundry.org/deployment: "mydeployment"
    quarks.cloudfoundry.org/provides: '{"name":"nats","type":"nats"}'
  name: nats-service
spec:
  ports:
  - port: 9099
    protocol: TCP
    targetPort: 9099
  selector:
    app: mynats
```

Using this service, I should be able to use `link("nats").address`, and I should get a value of `nats-service`.

This service selects for `Pods` that have the label `app: mynats`. The `instances` array should be populated using information from these pods.

If the secret is changed, consumers of the link are automatically restarted.

If the service is changed, or the list of pods selected by the service is changed, consumers of the link are automatically restarted.

## BOSH -> Native

In this case, the BOSH component is a provider, and the native component is a consumer.

The operator creates link Secrets for all providers in a BOSH deployment.

If a pod is annotated with the following:
  - `quarks.cloudfoundry.org/deployment: foo`
  - `quarks.cloudfoundry.org/consumes: '[{"name":"nats","type":"nats"}]'`
The operator will:
  - mutate the pod and mount the secret as `/quarks/link/DEPLOYMENT/link.yaml`

If link information changes, the operator will trigger an update (restart) of the deployment or statefulset owning the pod.
This can be done by updating the template of the pod using an annotation.

## Example

an Eirini Helm Chart

The OPI process of Eirini required the NATS password and IP.

```yaml
...
  template:
    metadata:
      quarks.cloudfoundry.org/deployment: {{ .Values.deploymentName }}
      quarks.cloudfoundry.org/consumes: '[{"name":"nats","type":"nats"}]'`
    spec:

```
and a CF-Deployment with Operator
Instance Groups:

- API
- Diego Cell
- Gorouter
- NATS
  provides: nats

