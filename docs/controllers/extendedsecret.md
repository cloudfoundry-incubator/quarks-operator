# ExtendedSecret

- [ExtendedSecret](#extendedsecret)
  - [Description](#description)
  - [Features](#features)
    - [Generated](#generated)
    - [Policies](#policies)
  - [Example Resource](#example-resource)

## Description

## Types

Currently ExtendedSecret supports to generate certificate, password, rsa, and ssh variables whose followed BOSH CLI generation options:

> See [detailed info](https://bosh.io/docs/variable-types)

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
  name: my-generated-secret
spec:
  # Type of the variable that ExtendedSecret supports
  type: certificate
  # Name of the Secret that stores this variable
  secretName: cf-deployment.uaa-ssl
  policy:
    recreateInterval: 3600s
  request:
    certificate:
      # The secret of CA private key
      CAKeyRef:
        Key: private_key
        Name: cf-deployment.uaa-ca
      # The secret of CA certificate
      CARef:
        Key: certificate
        Name: cf-deployment.uaa-ca
      # The Subject name of the certificate
      commonName: uaa.service.cf.internal
      # The subject alternative names
      alternativeNames:
      - uaa.service.cf.internal
      # If true, the ExtendedSecret will generate self-signed root CA certificate and private key
      isCA: false
```
