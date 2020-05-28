---
title: "Labels"
linkTitle: "Labels"
weight: 11
---

* `bdv1.LabelDeploymentName` "quarks.cloudfoundry.org/deployment-name"
* `bdv1.LabelDeploymentSecretType` "quarks.cloudfoundry.org/secret-type"
* `bdv1.LabelDeploymentVersion` "quarks.cloudfoundry.org/deployment-version"
* `bdv1.LabelInstanceGroupName` "quarks.cloudfoundry.org/instance-group-name"
* `bdv1.LabelReferencedJobName` "quarks.cloudfoundry.org/referenced-job-name"
* `qstsv1a1.LabelAZIndex` "quarks.cloudfoundry.org/az-index"
* `qstsv1a1.LabelAZName` "quarks.cloudfoundry.org/az-name"
* `qstsv1a1.LabelActivePod` "quarks.cloudfoundry.org/pod-active"
* `qstsv1a1.LabelPodOrdinal` "quarks.cloudfoundry.org/pod-ordinal"
* `qstsv1a1.LabelQStsName` "quarks.cloudfoundry.org/quarks-statefulset-name"
* `qsv1a1.LabelKind` "quarks.cloudfoundry.org/secret-kind"
* `delete` = pod
* `variableName`
* `app`

# Data From Tests

## ConfigMaps

### Secret Rotation ConfigMap

```
quarks.cloudfoundry.org/secret-rotation = yes
```

## QJob

```
quarks.cloudfoundry.org/deployment-name = bosh-manifest-two-instance-groups
delete=pod
quarks.cloudfoundry.org/deployment-version = 1
quarks.cloudfoundry.org/instance-group-name = nats-smoke-tests
```

### QJob Job

```
quarks.cloudfoundry.org/qjob-name = ig-bosh-manifest-two-instance-groups
```

### Pod from Job

```
delete=pod
quarks.cloudfoundry.org/qjob-name=dm-test
```

## QSTS

```
quarks.cloudfoundry.org/deployment-name = test
quarks.cloudfoundry.org/deployment-version = 1
quarks.cloudfoundry.org/instance-group-name = nats
```

### STS

```
quarks.cloudfoundry.org/az-index=0
quarks.cloudfoundry.org/deployment-name=test
quarks.cloudfoundry.org/instance-group-name=nats
quarks.cloudfoundry.org/quarks-statefulset-name=test-nats
```

## STS Pod

```
quarks.cloudfoundry.org/az-index=0
quarks.cloudfoundry.org/deployment-name=test
quarks.cloudfoundry.org/instance-group-name=nats
quarks.cloudfoundry.org/pod-active=active|true
quarks.cloudfoundry.org/pod-ordinal=0
quarks.cloudfoundry.org/quarks-statefulset-name=test-nats
```

## Service

```
quarks.cloudfoundry.org/az-index=0
quarks.cloudfoundry.org/deployment-name=test
quarks.cloudfoundry.org/instance-group-name=nats
quarks.cloudfoundry.org/pod-ordinal=0
```

## QSec

```
quarks.cloudfoundry.org/deployment-name = test
variableName = nats_password
```

### QSec Secret

```
quarks.cloudfoundry.org/secret-kind=generated
```

## Secrets

### WithOps Secrets

```
quarks.cloudfoundry.org/deployment-name=test
quarks.cloudfoundry.org/secret-type=with-ops
```

### Desired Manifest Secret

```
quarks.cloudfoundry.org/container-name=desired-manifest
quarks.cloudfoundry.org/deployment-name=test
quarks.cloudfoundry.org/entanglement=testdesired-manifest
quarks.cloudfoundry.org/referenced-job-name=instance-group-test
quarks.cloudfoundry.org/secret-kind=versionedSecret
quarks.cloudfoundry.org/secret-type=desired
quarks.cloudfoundry.org/secret-version=1
```

### BPM Secret

```
quarks.cloudfoundry.org/container-name=nats
quarks.cloudfoundry.org/deployment-name=test
quarks.cloudfoundry.org/entanglement=testbpmnats
quarks.cloudfoundry.org/remote-id=nats
quarks.cloudfoundry.org/secret-kind=versionedSecret
quarks.cloudfoundry.org/secret-type=bpm
quarks.cloudfoundry.org/secret-version=1
```

### IG Resolved Secret

```
quarks.cloudfoundry.org/container-name=nats
quarks.cloudfoundry.org/deployment-name=test
quarks.cloudfoundry.org/entanglement=testig-resolvednats
quarks.cloudfoundry.org/remote-id=nats
quarks.cloudfoundry.org/secret-kind=versionedSecret
quarks.cloudfoundry.org/secret-type=ig-resolved
quarks.cloudfoundry.org/secret-version=1
```

### Link Secret

```
quarks.cloudfoundry.org/container-name=nats
quarks.cloudfoundry.org/deployment-name=test
quarks.cloudfoundry.org/entanglement=link-test-nats-nats
quarks.cloudfoundry.org/remote-id=nats
```
