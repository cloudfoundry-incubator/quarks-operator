// Package boshmanifest contains text assets for BOSH manifests and ops files
package boshmanifest

// The manifests in this file are used in integration tests. They reference
// existing docker images and are deployable.

// Gora is a BOSH manifest for our gora test release, without SSL
const Gora = `---
releases:
- name: quarks-gora
  version: "0.0.15"
  url: ghcr.io/cloudfoundry-incubator
  stemcell:
    os: SLE_15_SP1
    version: 27.10-7.0.0_374.gb8e8e6af
instance_groups:
- name: quarks-gora
  instances: 2
  jobs:
  - name: quarks-gora
    release: quarks-gora
    properties:
      quarks-gora:
        port: 4222
      quarks:
        ports:
        - name: "quarks-gora"
          protocol: "TCP"
          internal: 4222
`

// BOSHManifestWithTwoInstanceGroups has two instance groups nats and route_registrar
const BOSHManifestWithTwoInstanceGroups = `---
name: bosh-manifest-two-instance-groups
releases:
- name: nats
  version: "33"
  url: ghcr.io/cloudfoundry-incubator
  stemcell:
    os: SLE_15_SP1
    version: 27.8-7.0.0_374.gb8e8e6af
- name: routing
  version: 0.198.0
  url: ghcr.io/cloudfoundry-incubator
  stemcell:
    os: SLE_15_SP1
    version: 27.8-7.0.0_374.gb8e8e6af
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
      quarks:
        ports:
        - name: "nats"
          protocol: "TCP"
          internal: 4222
        - name: "nats-routes"
          protocol: "TCP"
          internal: 4223
- name: route_registrar
  instances: 2
  jobs:
  - name: route_registrar
    release: routing
    properties:
      quarks:
        bpm:
          processes:
          - name: route_registrar
            executable: sleep
            args: ["1000"]
      route_registrar:
        routes: []
      nats:
        user: nats
        password: natSpa55w0rd
        port: 4222
        machines:
          - 192.168.52.123
      uaa:
        clients:
          gorouter:
            secret: no
        ca_cert: ""
        ssl:
          port: 8443
`

// NatsSmall is a small manifest to start nats, used in most integration tests
const NatsSmall = `---
name: test
releases:
- name: nats
  version: "33"
  url: ghcr.io/cloudfoundry-incubator
  stemcell:
    os: SLE_15_SP1
    version: 27.8-7.0.0_374.gb8e8e6af
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
      quarks:
        ports:
        - name: "nats"
          protocol: "TCP"
          internal: 4222
        - name: "nats-routes"
          protocol: "TCP"
          internal: 4223
`

// NatsExplicitVar is the same as NatsSmall, but with an additional explicit var
const NatsExplicitVar = `---
name: test
releases:
- name: nats
  version: "33"
  url: ghcr.io/cloudfoundry-incubator
  stemcell:
    os: SLE_15_SP1
    version: 27.8-7.0.0_374.gb8e8e6af
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
        nats_password: ((nats_password))
        debug: true
      quarks:
        ports:
        - name: "nats"
          protocol: "TCP"
          internal: 4222
        - name: "nats-routes"
          protocol: "TCP"
          internal: 4223
variables:
- name: nats_password
  type: password
`

// NatsAddOn is the same as NatsSmall, but with an addon bosh dns job
const NatsAddOn = `---
name: test
releases:
- name: nats
  version: "33"
  url: ghcr.io/cloudfoundry-incubator
  stemcell:
    os: SLE_15_SP1
    version: 27.8-7.0.0_374.gb8e8e6af
- name: bosh-dns-aliases
  url: https://bosh.io/d/github.com/cloudfoundry/bosh-dns-aliases-release?v=0.0.3
  version: 0.0.3
  sha1: b0d0a0350ed87f1ded58b2ebb469acea0e026ccc
addons:
- name: bosh-dns-aliases
  jobs:
  - name: bosh-dns-aliases
    release: bosh-dns-aliases
    properties:
      aliases:
      - domain: nats.service.cf.internal
        targets:
        - query: '*'
          instance_group: nats
          deployment: cf
          network: default
          domain: bosh
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
        nats_password: ((nats_password))
        debug: true
      quarks:
        ports:
        - name: "nats"
          protocol: "TCP"
          internal: 4222
        - name: "nats-routes"
          protocol: "TCP"
          internal: 4223
variables:
- name: nats_password
  type: password
`

// NatsSmallWithLinks has explicit BOSH links.
// It can be used in integration tests.
const NatsSmallWithLinks = `---
name: test
releases:
- name: nats
  version: "33"
  url: ghcr.io/cloudfoundry-incubator
  stemcell:
    os: SLE_15_SP1
    version: 27.8-7.0.0_374.gb8e8e6af
instance_groups:
- name: nats
  instances: 2
  jobs:
  - name: nats
    provides:
      nats: { shared: true, as: nutty_nuts }
    release: nats
    properties:
      nats:
        user: admin
        password: changeme
        debug: true
      quarks:
        ports:
        - name: "nats"
          protocol: "TCP"
          internal: 4222
        - name: "nats-routes"
          protocol: "TCP"
          internal: 4223
`

// NatsWithSmokeTest has a link, which is provided by another instance group
const NatsWithSmokeTest = `---
name: test
releases:
- name: nats
  version: "33"
  url: ghcr.io/cloudfoundry-incubator
  stemcell:
    os: SLE_15_SP1
    version: 27.8-7.0.0_374.gb8e8e6af
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
      quarks:
        ports:
        - name: "nats"
          protocol: "TCP"
          internal: 4222
        - name: "nats-routes"
          protocol: "TCP"
          internal: 4223
    provides:
      nats:
        as: nats
- name: nats-smoke-tests
  instances: 1
  lifecycle: auto-errand
  jobs:
  - name: smoke-tests
    release: nats
    consumes:
      nats: {from: nats}
`

// NatsSmallWithPatch is a manifest that patches the prestart hook to loop forever
// It can be used in integration tests.
const NatsSmallWithPatch = `---
name: test
releases:
- name: nats
  version: "33"
  url: ghcr.io/cloudfoundry-incubator
  stemcell:
    os: SLE_15_SP1
    version: 27.8-7.0.0_374.gb8e8e6af
instance_groups:
- name: nats
  instances: 1
  jobs:
  - name: nats
    release: nats
    properties:
      nats:
        user: admin
        password: changeme
        debug: true
      quarks:
        pre_render_scripts:
          jobs:
          - |
            cd /var/vcap
            ls -lahR
            tee -a /var/vcap/all-releases/jobs-src/nats/nats/templates/pre-start.erb << EOT
            while :
            do
              echo "this file was patched"
              sleep 1
            done
            EOT
        ports:
        - name: "nats"
          protocol: "TCP"
          internal: 4222
        - name: "nats-routes"
          protocol: "TCP"
          internal: 4223
`

// NatsSmokeTestWithExternalLinks has explicit BOSH links.
// It can be used in integration tests.
const NatsSmokeTestWithExternalLinks = `---
name: test
releases:
- name: nats
  version: "33"
  url: ghcr.io/cloudfoundry-incubator
  stemcell:
    os: SLE_15_SP1
    version: 27.8-7.0.0_374.gb8e8e6af
instance_groups:
- name: nats-smoke-tests
  instances: 1
  lifecycle: auto-errand
  jobs:
  - name: smoke-tests
    release: nats
    consumes:
      nats: {from: nats}
`

// Drains is a small manifest with jobs that include drain scripts
// It can be used in integration tests.
const Drains = `---
name: my-manifest
releases:
- name: quarks-gora
  version: "0.0.17"
  url: ghcr.io/cloudfoundry-incubator
  stemcell:
    os: SLE_15_SP2
    version: 29.6-7.0.0_374.gb8e8e6af
instance_groups:
- name: drains
  instances: 1
  jobs:
  - name: failing-drain-job
    release: quarks-gora
  - name: delaying-drain-job
    release: quarks-gora
`

// BPMRelease utilizing the test server to open two tcp ports
// It can be used in integration tests.
const BPMRelease = `
name: test-bdpl

releases:
- name: bpm
  version: 1.1.7
  url: ghcr.io/cloudfoundry-incubator
  stemcell:
    os: SLE_15_SP1
    version: 27.8-7.0.0_374.gb8e8e6af

instance_groups:
- name: bpm
  instances: 1
  jobs:
  - name: test-server
    release: bpm
    properties:
      quarks:
        ports:
        - name: test-server
          protocol: TCP
          internal: 1337
        - name: alt-test-server
          protocol: TCP
          internal: 1338
  persistent_disk: 1024
  persistent_disk_type: ((operator_test_storage_class))
`

// CFRouting BOSH release is being tested for BOSH pre hook
// It can be used in integration tests.
const CFRouting = `
name: routing

releases:
- name: routing
  version: 0.198.0
  url: ghcr.io/cloudfoundry-incubator
  stemcell:
    os: SLE_15_SP1
    version: 27.8-7.0.0_374.gb8e8e6af

instance_groups:
- name: route_registrar
  instances: 2
  jobs:
  - name: route_registrar
    release: routing
    properties:
      quarks:
        bpm:
          processes:
          - name: route_registrar
            executable: sleep
            args: ["1000"]
      route_registrar:
        routes: []
      nats:
        user: nats
        password: natSpa55w0rd
        port: 4222
        machines:
          - 192.168.52.123
      uaa:
        clients:
          gorouter:
            secret: no
        ca_cert: ""
        ssl:
          port: 8443
`

// Diego BOSH release is being tested for BPM pre start hook
// It can be used in integration tests.
const Diego = `
  name: diego

  releases:
  - name: diego
    version: 2.44.0
    url: ghcr.io/cloudfoundry-incubator
    stemcell:
      os: SLE_15_SP1
      version: 27.8-7.0.0_374.gb8e8e6af

  instance_groups:
  - name: file_server
    instances: 2
    jobs:
    - name: file_server
      release: diego
      properties:
        bpm:
          enabled: true
        enable_consul_service_registration: false
        set_kernel_parameters: false
`
