---
title: "Naming Conventions"
linkTitle: "Naming Conventions"
weight: 11
---

- [Naming Conventions](#naming-conventions)
  - [Kubernetes Resources](#kubernetes-resources)
    - [Kubernetes Services](#kubernetes-services)

## Kubernetes Resources

Kube names can only consist of lowercase alphanumeric characters, and the character `"-"`.
All `"_"` characters are replaced with `"-"`. All other non-alphanumeric characters are removed.

The `name` cannot start or end with a `"-"`. These characters are trimmed.

Names are also restricted to 63 characters in length, so if a generated name exceeds 63 characters, it should be recalculated as:

```text
name=<INSTANCE_GROUP_NAME>-<INDEX><DEPLOYMENT_NAME>

<name trimmed to 31 characters><md5 hash of name>
```

### Kubernetes Services

The same check needs to apply to the entire address of a `Service`. If an entire address is longer than 253 characters, the `servicename` is trimmed until there's enough room for the MD5 hash. If it's not possible to include the hash (`KUBE_NAMESPACE` and `KUBE_SERVICE_DOMAIN` and the dots are 221 characters or more), an error is thrown.
