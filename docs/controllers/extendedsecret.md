# ExtendedSecret

- [ExtendedSecret](#extendedsecret)
  - [Description](#description)
  - [Types](#types)
  - [Features](#features)
    - [Generated](#generated)
    - [Policies](#policies)
  - [`ExtendedSecret` Examples](#extendedsecret-examples)

## Description

## Types

`ExtendedSecret` supports generating the following:

- certificates
- passwords
- rsa keys

> **Note:**
>
> You can find more details in the [BOSH docs](https://bosh.io/docs/variable-types).

## Features

### Generated

A pluggable implementation for generating certificates and passwords.

### Policies

The developer can specify policies for rotation (e.g. automatic or not) and how secrets are created (e.g. password complexity, certificate expiration date, etc.).

## `ExtendedSecret` Examples

See https://github.com/cloudfoundry-incubator/cf-operator/tree/master/docs/examples/extended-secret
