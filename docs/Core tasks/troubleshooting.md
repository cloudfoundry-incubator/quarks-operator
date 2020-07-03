---
title: "Troubleshooting"
linkTitle: "Troubleshooting"
weight: 4
description: >
  Troubleshooting notes for Quarks-operator
---

### Cluster CA

The `cf-operator` assumes that the cluster root CA is also used for signing CSRs via the certificates.k8s.io API and will embed this CA in the generated certificate secrets. If your cluster is set up to use a different cluster-signing CA the generated certificates will have the wrong CA embedded. See https://kubernetes.io/docs/tasks/tls/managing-tls-in-a-cluster/ for more information on cluster trust.

### Recovering from a crash

If the operator pod crashes, it cannot be restarted in the same namespace before the existing mutating webhook configuration for that namespace is removed.
The operator uses mutating webhooks to modify pods on the fly and Kubernetes fails to create pods if the webhook server is unreachable.
The webhook configurations are installed cluster wide and don't belong to a single namespace, just like custom resources.

To remove the webhook configurations for the cf-operator namespace run:

```bash
CF_OPERATOR_NAMESPACE=cf-operator
kubectl delete mutatingwebhookconfiguration "cf-operator-hook-$CF_OPERATOR_NAMESPACE"
kubectl delete validatingwebhookconfiguration "cf-operator-hook-$CF_OPERATOR_NAMESPACE"
```

From **Kubernetes 1.15** onwards, it is possible to instead patch the webhook configurations for the cf-operator namespace via:

```bash
CF_OPERATOR_NAMESPACE=cf-operator
kubectl patch mutatingwebhookconfigurations "cf-operator-hook-$CF_OPERATOR_NAMESPACE" -p '
webhooks:
- name: mutate-pods.quarks.cloudfoundry.org
  objectSelector:
    matchExpressions:
    - key: name
      operator: NotIn
      values:
      - "cf-operator"
'
```