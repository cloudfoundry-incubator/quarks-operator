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
   3. [Relationship with the BDPL component](#relationship-with-the-bdpl-component)
   4. [`QuarksSecret` Examples](#`quarkssecret`-examples)

## Description

An QuarksSecret generates passwords, keys and certificates and stores them in Kubernetes secrets.

## QuarksSecret Component

The **QuarksSecret** component is a categorization of a set of controllers, under the same group. Inside the **QuarksSecret** component, we have a set of 2 controllers together with one separate reconciliation loop per controller.

Figure 1, illustrates the component and associated set of controllers.

![qsec-component-flow](quarks_eseccomponent_flow.png)
*Fig. 1: The QuarksSecret component*

### **_QuarksSecret Controller_**

![qsec-controller-flow](quarks_eseccontroller_flow.png)
*Fig. 2: The QuarksSecret controller*

The **QuarksSecret** Controller will get a list of all variables referenced in a BOSH manifest with ops files applied, and will use this list of variables to generate the pertinent `QuarksSecret` instances.

#### Watches in quarks secret controller

- `QuarksSecret`: Creation

#### Reconciliation in quarks secret controller

- generates Kubernetes secret of specific types(see Types under Highlights).
- generate a Certificate Signing Request against the cluster API.

#### Highlights in quarks secret controller

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

##### Policies

The developer can specify policies for rotation (e.g. automatic or not ) and how secrets are created (e.g. password complexity, certificate expiration date, etc.).

##### Auto-approving Certificates

A certificate `QuarksSecret` can be signed by the Kube API Server. The **QuarksSecret** Controller is responsible for generating the certificate signing request:

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

#### Watches in CSR controller

- `Certificate Signing Request`: Creation

#### Reconciliation in CSR controller

- once the request is approved by Kubernetes API, will generate a certificate stored in a Kubernetes secret, that is recognized by the cluster.

#### Highlights in CSR controller

The CertificateSigningRequest controller watches for `CertificateSigningRequest` and approves `QuarksSecret`-owned CSRs and persists the generated certificate.

## Relationship with the BDPL component

![bdpl-qjob-relationship](quarks_gvc_and_esec_flow.png)
*Fig. 4: Relationship between the Generated V.  controller and the QuarksSecret component*

Figure 4 illustrates the interaction of the **Generated Variables** Controller with the **QuarksSecret** Controller. When reconciling, the Generated Variables Controller lists all variables of a BOSH manifest(basically all BOSH variables) and generates an `QuarksSecret` instance per variable, which will trigger the **QuarksSecret** Controller.

## `QuarksSecret` Examples

See https://github.com/cloudfoundry-incubator/cf-operator/tree/master/docs/examples/quarks-secret
