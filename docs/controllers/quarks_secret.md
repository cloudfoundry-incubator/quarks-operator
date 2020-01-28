# QuarksSecret

1. [QuarksSecret](#quarkssecret)
   1. [Description](#description)
   2. [QuarksSecret Component](#quarkssecret-component)
      1. [QuarksSecret Controller](#_quarkssecret-controller_)
         1. [Watches](#watches-in-quarks-secret-controller)
         2. [Reconciliation](#reconciliation-in-quarks-secret-controller)
         3. [Types](#types)
         4. [Policies](#policies)
         5. [Auto-approving Certificates](#auto-approving-certificates)
      2. [CertificateSigningRequest Controller](#_certificatesigningrequest-controller_)
         1. [Watches](#watches-in-csr-controller)
         2. [Reconciliation](#reconciliation-in-csr-controller)
         3. [Highlights](#highlights-in-csr-controller)
      3. [SecretRotation Controller](#_secretrotation-controller_)
         1. [Watches](#watches-in-secret-rotation-controller)
         2. [Reconciliation](#reconciliation-in-secret-rotation-controller)
   3. [Relationship with the BDPL component](#relationship-with-the-bdpl-component)
   4. [`QuarksSecret` Examples](#`quarkssecret`-examples)

## Description

An QuarksSecret generates passwords, keys and certificates and stores them in Kubernetes secrets.

## QuarksSecret Component

The **QuarksSecret** component consists of three controllers, each with a separate reconciliation loop.

Figure 1, illustrates the component and associated set of controllers.

![qsec-component-flow](quarks_eseccomponent_flow.png)
*Fig. 1: The QuarksSecret component*

### **_QuarksSecret Controller_**

![qsec-controller-flow](quarks_eseccontroller_flow.png)
*Fig. 2: The QuarksSecret controller*


#### Watches in Quarks Secret Controller

- `QuarksSecret`: Creation
- `QuarksSecret`: Updates if `.status.generated` is false

#### Reconciliation in Quarks Secret Controller

- generates Kubernetes secret of specific types(see Types under Highlights).
- generate a Certificate Signing Request against the cluster API.
- sets `.status.generated` to `true`, to avoid re-generation and allow secret rotation.

#### Highlights in Quarks Secret Controller

##### Types

Depending on the `spec.type`, `QuarksSecret` supports generating the following:

| Secret Type                     | spec.type     | certificate.signerType | certificate.isCA    |
| ------------------------------- | ------------- | ---------------------- | ------------------- |
| `passwords`                     | `password`    | not set                | not set             |
| `rsa keys`                      | `rsa`         | not set                | not set             |
| `ssh keys`                      | `ssh`         | not set                | not set             |
| `self-signed root certificates` | `certificate` | `local`                | `true`              |
| `self-signed certificates`      | `certificate` | `local`                | `false`             |
| `cluster-signed certificates`   | `certificate` | `cluster`              | `false`             |

> **Note:**
>
> You can find more details in the [BOSH docs](https://bosh.io/docs/variable-types).

##### Auto-approving Certificates

A certificate `QuarksSecret` can be signed by the Kubernetes API Server. The **QuarksSecret** Controller is responsible for generating the certificate signing request:

```yaml
apiVersion: certificates.k8s.io/v1beta1
kind: CertificateSigningRequest
metadata:
  name: generate-certificate
spec:
  request: ((encoded-cert-signing-request))
  usages:
  - digital signature
  - key encipherment
```

### **_CertificateSigningRequest Controller_**

![certsr-controller-flow](quarks_certsrcontroller_flow.png)
*Fig. 3: The CertificateSigningRequest controller*

#### Watches in CSR Controller

- `Certificate Signing Request`: Creation

#### Reconciliation in CSR Controller

- once the request is approved by Kubernetes API, will generate a certificate stored in a Kubernetes secret, that is recognized by the cluster.

#### Highlights in CSR Controller

The CertificateSigningRequest controller watches for `CertificateSigningRequest` and approves `QuarksSecret`-owned CSRs and persists the generated certificate.

### **_SecretRotation Controller_**

The secret rotation controller watches for a rotation config map and re-generates all the listed `QuarksSecrets`.

#### Watches in Secret Rotation Controller

- `ConfigMap`: Creation of a config map, which has the `secret-rotation` label.

#### Reconciliation in Secret Rotation Controller

- Will read the array of `QuarksSecret` names from the JSON under the config map key `secrets`.
- Skip `QuarksSecret` where `.status.generated` is `false`, as these might be under control of the user.
- Set `.status.generated` for each named `QuarksSecret` to `false`, to trigger re-creation of the corresponding secret.

## Relationship With the BDPL Component

All explicit variables of a BOSH manifest will be created as `QuarksSecret` instances, which will trigger the **QuarksSecret** Controller.
This will create corresponding secrets. If the user decides to change a secret, the `.status.generated` field in the corresponding `QuarksSecret` should be set to `false`, to protect against overwriting.

## `QuarksSecret` Examples

See https://github.com/cloudfoundry-incubator/cf-operator/tree/master/docs/examples/quarks-secret
