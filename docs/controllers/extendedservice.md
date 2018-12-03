# ExtendedService

- [ExtendedService](#extendedservice)
    - [Description](#description)
    - [Features](#features)
        - [Port Ranges](#port-ranges)
        - [Service Alias](#service-alias)
        - [Services with Active/Passive support](#services-with-activepassive-support)
    - [Example Resource](#example-resource)

## Description

## Features

### Port Ranges

Support for port ranges in services.

### Service Alias

Easy support for configuring aliases. [`ExternalName`]( https://kubernetes.io/docs/concepts/services-networking/service/#externalname) entities could be useful for this.

### Services with Active/Passive support

The developer can implement an active/passive model with pods that **are ready**.

## Example Resource

```yaml
---
```
