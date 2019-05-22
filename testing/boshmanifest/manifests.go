// Package boshmanifest contains text assets for BOSH manifests and ops files
package boshmanifest

const Default = `name: foo-deployment
stemcells:
- alias: default
  os: opensuse-42.3
  version: 28.g837c5b3-30.263-7.0.0_234.gcd7d1132
instance_groups:
- name: redis-slave
  instances: 2
  lifecycle: errand
  azs: [z1, z2]
  jobs:
  - name: redis-server
    release: redis
    properties: {}
  vm_type: medium
  stemcell: default
  persistent_disk_type: medium
  networks:
  - name: default
  properties:
    foo:
      app_domain: "((app_domain))"
    bosh_containerization:
      ports:
      - name: "redis"
        protocol: "TCP"
        internal: 6379
- name: diego-cell
  azs:
  - z1
  - z2
  instances: 2
  lifecycle: service
  vm_type: small-highmem
  vm_extensions:
  - 100GB_ephemeral_disk
  stemcell: default
  networks:
  - name: default
  jobs:
  - name: cflinuxfs3-rootfs-setup
    release: cflinuxfs3
    properties:
      foo:
        domain: "((system_domain))"
      bosh_containerization:
        ports:
        - name: "rep-server"
          protocol: "TCP"
          internal: 1801
variables:
- name: "adminpass"
  type: "password"
  options: {is_ca: true, common_name: "some-ca"}
releases:
- name: cflinuxfs3
  version: 0.62.0
  url: hub.docker.com/cfcontainerization
  sha1: 6466c44827c3493645ca34b084e7c21de23272b4
  stemcell:
    os: opensuse-15.0
    version: 28.g837c5b3-30.263-7.0.0_233.gde0accd0
- name: redis
  version: 36.15.0
  url: hub.docker.com/cfcontainerization
  sha1: 6466c44827c3493645ca34b084e7c21de23272b4`

const Small = `---
name: my-manifest
releases:
- name: nats
  version: "26"
  url: docker.io/cfcontainerization
  stemcell:
    os: opensuse-42.3
    version: 30.g9c91e77-30.80-7.0.0_257.gb97ced55
instance_groups:
- name: nats
  instances: 2
  jobs:
  - name: nats
    release: nats
    properties:
      nats:
        user: admin
        password: changeme
        debug: true
      bosh_containerization:
        ports:
        - name: "nats"
          protocol: "TCP"
          internal: 4222
        - name: "nats-routes"
          protocol: "TCP"
          internal: 4223
`

const Elaborated = `name: foo-deployment
stemcells:
- alias: default
  os: opensuse-42.3
  version: 28.g837c5b3-30.263-7.0.0_234.gcd7d1132
instance_groups:
- name: redis-slave
  instances: 2
  lifecycle: errand
  azs: [z1, z2]
  jobs:
  - name: redis-server
    release: redis
    properties:
      password: foobar
    provides:
      redis: {as: redis-server}
  vm_type: medium
  stemcell: default
  persistent_disk_type: medium
  networks:
  - name: default
- name: diego-cell
  azs:
  - z1
  - z2
  instances: 2
  lifecycle: service
  vm_type: small-highmem
  vm_extensions:
  - 100GB_ephemeral_disk
  stemcell: default
  networks:
  - name: default
  jobs:
  - name: cflinuxfs3-rootfs-setup
    release: cflinuxfs3
variables:
- name: "adminpass"
  type: "password"
  options: {is_ca: true, common_name: "some-ca"}
releases:
- name: cflinuxfs3
  version: 0.62.0
  url: hub.docker.com/cfcontainerization
  sha1: 6466c44827c3493645ca34b084e7c21de23272b4
  stemcell:
    os: opensuse-15.0
    version: 28.g837c5b3-30.263-7.0.0_233.gde0accd0
- name: redis
  version: 36.15.0
  url: hub.docker.com/cfcontainerization
  sha1: 6466c44827c3493645ca34b084e7c21de23272b4`

const WithProviderAndConsumer = `---
name: cf
manifest_version: v7.7.0
instance_groups:
- name: doppler
  azs:
  - z1
  - z2
  instances: 4
  vm_type: minimal
  stemcell: default
  networks:
  - name: default
  jobs:
  - name: doppler
    release: loggregator
    provides:
      doppler: {as: doppler, shared: true}
    properties:
      doppler:
        grpc_port: 7765
      metron_endpoint:
        host: foobar.com
      loggregator:
        tls:
          ca_cert: "((loggregator_ca.certificate))"
          doppler:
            cert: "((loggregator_tls_doppler.certificate))"
            key: "((loggregator_tls_doppler.private_key))"
- name: log-api
  azs:
  - z1
  - z2
  instances: 2
  vm_type: minimal
  stemcell: default
  update:
    serial: true
  networks:
  - name: default
  jobs:
  - name: loggregator_trafficcontroller
    release: loggregator
    consumes:
      doppler: {from: doppler}
    properties:
      uaa:
        internal_url: https://uaa.service.cf.internal:8443
        ca_cert: "((uaa_ca.certificate))"
      doppler:
        grpc_port: 6060
      loggregator:
        tls:
          cc_trafficcontroller:
            cert: "((loggregator_tls_cc_tc.certificate))"
            key: "((loggregator_tls_cc_tc.private_key))"
          ca_cert: "((loggregator_ca.certificate))"
          trafficcontroller:
            cert: "((loggregator_tls_tc.certificate))"
            key: "((loggregator_tls_tc.private_key))"
        uaa:
          client_secret: "((uaa_clients_doppler_secret))"
      system_domain: "((system_domain))"
      ssl:
        skip_cert_verify: true
      cc:
        internal_service_hostname: "cloud-controller-ng.service.cf.internal"
        tls_port: 9023
        mutual_tls:
          ca_cert: "((service_cf_internal_ca.certificate))"
releases:
- name: loggregator
  url: https://bosh.io/d/github.com/cloudfoundry/loggregator-release?v=105.0
  version: "105.0"
  sha1: d0bed91335aaac418eb6e8b2be13c6ecf4ce7b90
stemcells:
- alias: default
  os: ubuntu-xenial
  version: "250.17"
`

const WithBPMInfo = `---
name: foo-deployment
stemcells:
- alias: default
  os: opensuse-42.3
  version: 28.g837c5b3-30.263-7.0.0_234.gcd7d1132
instance_groups:
- name: redis-slave
  instances: 2
  lifecycle: errand
  azs: [z1, z2]
  jobs:
  - name: redis-server
    release: redis
    properties:
      foo:
        app_domain: "((app_domain))"
      bosh_containerization:
        ports:
        - name: "redis"
          protocol: "TCP"
          internal: 6379
        bpm:
          processes:
          - name: redis
            executable: /another/command
            limits:
            open_files: 100000
  vm_type: medium
  stemcell: default
  persistent_disk_type: medium
  networks:
  - name: default
releases:
- name: redis
  version: 36.15.0
  url: hub.docker.com/cfcontainerization
  sha1: 6466c44827c3493645ca34b084e7c21de23272b4
`

const WithMultiBPMProcesses = `---
name: my-manifest
releases:
- name: fake-release
  version: "26"
  url: docker.io/cfcontainerization
  stemcell:
    os: opensuse-42.3
    version: 30.g9c91e77-30.80-7.0.0_257.gb97ced55
- name: dummy-release
  version: "1"
  url: docker.io/cfcontainerization
  stemcell:
    os: dummy-os-1
    version: 1
instance_groups:
- name: fake-ig-1
  instances: 2
  lifecycle: errand
  jobs:
  - name: fake-errand-a
    release: fake-release
    properties:
      fake-release:
        user: admin
        password: changeme
  - name: fake-errand-b
    release: fake-release
- name: fake-ig-2
  instances: 3
  jobs:
  - name: fake-job-a
    release: dummy-release
  - name: fake-job-b
    release: fake-release
  - name: fake-job-c
    release: fake-release
- name: fake-ig-3
  instances: 1
  jobs:
  - name: fake-job-a
    release: dummy-release
  - name: fake-job-b
    release: fake-release
  - name: fake-job-c
    release: fake-release
  - name: fake-job-d
    release: fake-release
`

const BPMRelease = `
name: bpm

releases:
- name: bpm
  version: 1.0.4
  url: docker.io/cfcontainerization
  stemcell:
    os: opensuse-42.3
    version: 36.g03b4653-30.80-7.0.0_316.gcf9fe4a7

instance_groups:
- name: bpm
  instances: 1
  jobs:
  - name: test-server
    release: bpm
    properties:
      bosh_containerization:
        ports:
        - name: test-server
          protocol: TCP
          internal: 1337
        - name: alt-test-server
          protocol: TCP
          internal: 1338
`
