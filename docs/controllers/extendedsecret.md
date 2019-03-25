# ExtendedSecret

- [ExtendedSecret](#extendedsecret)
  - [Description](#description)
  - [Features](#features)
    - [Generated](#generated)
    - [Policies](#policies)
  - [Example Resource](#example-resource)

## Description

## Features

### Generated

A pluggable implementation for generating certificates and passwords.

### Policies

The developer can specify policies for rotation (e.g. automatic or not) and how secrets are created (e.g. password complexity, certificate expiration date, etc.).

## Example Resource

```yaml
---
apiVersion: fissile.cloudfoundry.org/v1alpha1
kind: ExtendedSecret
metadata:
  name: mygeneratedsecret
spec:
  secretName: secretToBeCreated
  policy:
    recreateInterval: 3600s
  request:
    # one of the following
    # the specific contents for each type of request need to be determined by the definitions here:
    # https://github.com/cloudfoundry-incubator/cf-operator/blob/master/pkg/credsgen/generator.go#L43
    # in the case where we need to reference a CA, that should be done by referencing a Secret
    password:
    certificate:
    sshKey:
    rsaKey:
```