// Package boshmanifest contains text assets for BOSH manifests and ops files
package boshmanifest

// NatsSmall is a small manifest to start nats, used in most integration tests
const NatsSmall = `---
name: test
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
      quarks:
        ports:
        - name: "nats"
          protocol: "TCP"
          internal: 4222
        - name: "nats-routes"
          protocol: "TCP"
          internal: 4223
`

// NatsSmallWithLinks has explicit BOSH links.
// It can be used in integration tests.
const NatsSmallWithLinks = `---
name: test
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
    provides:
      nats: { shared: true, as: nuts }
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

// NatsSmallWithPatch is a manifest that patches the prestart hook to loop forever
// It can be used in integration tests.
const NatsSmallWithPatch = `---
name: test
releases:
- name: nats
  version: "26"
  url: docker.io/cfcontainerization
  stemcell:
    os: opensuse-42.3
    version: 30.g9c91e77-30.80-7.0.0_257.gb97ced55
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

// Drains is a small manifest with jobs that include drain scripts
// It can be used in integration tests.
const Drains = `---
name: my-manifest
releases:
- name: cf-operator-testing
  version: "0.0.6"
  url: docker.io/cfcontainerization
  stemcell:
    os: opensuse-42.3
    version: 36.g03b4653-30.80-7.0.0_346.ge9dd9ff3
instance_groups:
- name: drains
  instances: 1
  jobs:
  - name: failing-drain-job
    release: cf-operator-testing
  - name: delaying-drain-job
    release: cf-operator-testing
`

// BPMRelease utilizing the test server to open two tcp ports
// It can be used in integration tests.
const BPMRelease = `
name: test-bdpl

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
  version: 0.188.0
  url: docker.io/cfcontainerization
  stemcell:
    os: opensuse-42.3
    version: 36.g03b4653-30.80-7.0.0_332.g0d8469bb

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
    version: 2.32.0
    url: docker.io/cfcontainerization
    stemcell:
      os: opensuse-42.3
      version: 36.g03b4653-30.80-7.0.0_332.g0d8469bb

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
