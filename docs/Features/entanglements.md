---
title: "Entanglements"
linkTitle: "Entanglements"
weight: 11
---

Also known as "Quarks Links" - they provide a way to share/discover information between BOSH and Kube Native components.

## Using k8s Native Values in BOSH Deployments (Native -> BOSH)

In this case, the native component is a provider, and the BOSH component is a consumer.

We construct link information from the native resources like this:

| BOSH Link           | Native  | Description                                                                                              |
| ------------------- | ------- | -------------------------------------------------------------------------------------------------------- |
| address             | Service | DNS address of a k8s *service* annotated  `quarks.cloudfoundry.org/provides = LINK_NAME`                 |
| azs                 | N/A     | not supported                                                                                            |
| properties          |         | properties retrieved from a *secret* annotated `quarks.cloudfoundry.org/provides = LINK_NAME`            |
| instances.name      | Pod     | name of *pod* selected by the k8s *service* that's annotated `quarks.cloudfoundry.org/provides = LINK_NAME` |
| instances.id        | Pod     | *pod* uid                                                                                                |
| instances.index     | Pod     | set to a value 0-(pod replica count)                                                                     |
| instances.az        | N/A     | not supported                                                                                            |
| instances.address   | Pod     | ip of *pod*                                                                                              |
| instances.bootstrap | Pod     | set to true if index == 0                                                                                |

> If multiple secrets or services are found with the same link information, the operator should error

### Example (Native -> BOSH)

When a job consumes a link, it will have a section like this in the in its job spec (`job.MF`), e.g. the nats release:

```yaml
consumes:
- name: nats
  type: nats
```

We can create the following k8s secret to fulfill the link:

```yaml
kind: Secret
metadata:
  name: secretlink
  labels:
    quarks.cloudfoundry.org/deployment-name: "mydeployment"
  annotations:
    quarks.cloudfoundry.org/provides: '{"name":"nats","type":"nats"}'
spec:
  data:
    link: |
      nats.user: myuser
      nats.password: mysecret
```

Using this secret, the nats release can use `link("nats").p("password")` in its eruby templates.

```eruby
"<%= p("nats.password") %>"
```

Furthermore, if there is a matching k8s service, it will be used in the link:

```yaml
apiVersion: v1
kind: Service
metadata:
  labels:
    quarks.cloudfoundry.org/deployment-name: "mydeployment"
  annotations:
    quarks.cloudfoundry.org/link-provider-name: nats
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

## Using BOSH Variables in k8s Pods (BOSH -> Native)

In this case, the BOSH component is a provider, and the native component is a consumer.
The native component is pod, which might belong to a deployment or statefulset.

The operator creates link secrets for all providers in a BOSH deployment. Each secret contains a flattened map with the provided properties:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: link-test-nats-nats
  annotations:
    quarks.cloudfoundry.org/restart-on-update: "true"
  labels:
    quarks.cloudfoundry.org/entanglement: link-test-nats-nats
data:
  nats.password: YXBwYXJlbnRseSwgeW91Cg==
  nats.port: aGF2ZSB0b28K
  nats.user: bXVjaCB0aW1lCg==
```

If a pod is annotated with the following:

- `quarks.cloudfoundry.org/deployment: foo`
- `quarks.cloudfoundry.org/consumes: '[{"name":"nats","type":"nats"}]'`

The operator will mutate the pod to:

- mount the link secrets as `/quarks/link/DEPLOYMENT/<type>-<name>/<key>`
- add an environment variable for each key in the secret data mapping: `LINK_<key>`

The `<name>` and `<type>` are the respective link type and name. For example, the nats release uses `nats` for both the name and the type of the link. The `<key>` describes the BOSH property, flattened (dot-style), for example `nats.password`. The key name is modified to be upper case and without dots in the context of an environment variable, therefore `nats.password` becomes `LINK_NATS_PASSWORD` in the container.

If link information changes, the operator will trigger an update (restart) of the deployment or statefulset owning the pod.
This can be done by updating the template of the pod using an annotation.

### Example (BOSH -> Native)

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

and a CF-Deployment with Operator, which has the following instance groups:
- API
- Diego Cell
- Gorouter
- NATS
  `provides: nats`
