# QuarksSecret

- [QuarksSecret](#quarkssecret)
  - [Description](#description)
  - [QuarksSecret Component](#quarkssecret-component)
    - [**_QuarksSecret Controller_**](#quarkssecret-controller)
      - [Watches in Quarks Secret Controller](#watches-in-quarks-secret-controller)
      - [Reconciliation in Quarks Secret Controller](#reconciliation-in-quarks-secret-controller)
      - [Highlights in Quarks Secret Controller](#highlights-in-quarks-secret-controller)
        - [Types](#types)
        - [Auto-approving Certificates](#auto-approving-certificates)
        - [Copies](#copies)
    - [**_CertificateSigningRequest Controller_**](#certificatesigningrequest-controller)
      - [Watches in CSR Controller](#watches-in-csr-controller)
      - [Reconciliation in CSR Controller](#reconciliation-in-csr-controller)
      - [Highlights in CSR Controller](#highlights-in-csr-controller)
    - [**_SecretRotation Controller_**](#secretrotation-controller)
      - [Watches in Secret Rotation Controller](#watches-in-secret-rotation-controller)
      - [Reconciliation in Secret Rotation Controller](#reconciliation-in-secret-rotation-controller)
  - [Relationship With the BDPL Component](#relationship-with-the-bdpl-component)
  - [`QuarksSecret` Examples](#quarkssecret-examples)

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

| Secret Type                     | spec.type     | certificate.signerType | certificate.isCA |
| ------------------------------- | ------------- | ---------------------- | ---------------- |
| `passwords`                     | `password`    | not set                | not set          |
| `rsa keys`                      | `rsa`         | not set                | not set          |
| `ssh keys`                      | `ssh`         | not set                | not set          |
| `self-signed root certificates` | `certificate` | `local`                | `true`           |
| `self-signed certificates`      | `certificate` | `local`                | `false`          |
| `cluster-signed certificates`   | `certificate` | `cluster`              | `false`          |

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

##### Copies

The `QuarksSecret` controller can create copies of a generated secret across multiple namespaces, as long as the target secrets (that live in a namespace other than the namespace of the `QuarksSecret`) already exist, and have an annotation of:

```text
quarks.cloudfoundry.org/secret-copy-of: NAMESPACE/generate-password-with-copies
```

as well as the usual label for generated secrets:

```text
quarks.cloudfoundry.org/secret-kind: generated
```

This ensures that the creator of the `QuarksSecret` must have access to the copy target namespace.

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
