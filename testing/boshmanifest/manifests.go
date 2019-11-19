// Package boshmanifest contains text assets for BOSH manifests and ops files
package boshmanifest

// Default is the default BOSH manifest for tests
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
  update:
    canary_watch_time: 20000-1200000
  networks:
  - name: default
  properties:
    foo:
      app_domain: "((app_domain))"
  env:
    bosh:
      agent:
        settings:
          labels:
            custom-label: foo
          annotations:
            custom-annotation: bar
- name: diego-cell
  azs:
  - z1
  - z2
  instances: 2
  lifecycle: service
  persistent_disk: 1024
  persistent_disk_type: "standard"
  vm_type: small-highmem
  update:
    canary_watch_time: 20000-1200000
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
      quarks:
        run:
          healthcheck:
            test-server:
              readiness:
                exec:
                  command:
                  - "curl --silent --fail --head http://${HOSTNAME}:8080/health"
              liveness:
                exec:
                  command:
                  - "curl --silent --fail --head http://${HOSTNAME}:8080"
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

// WithAddons is a BOSH manifest with addons for tests
const WithAddons = `name: foo-deployment
addons:
- name: test
  include:
    stemcell:
    - os: opensuse-42.3
    - os: opensuse-15.0
  jobs:
  - name: addon-job
    release: redis
- name: test2
  include:
    stemcell:
    - os: opensuse-42.3
  exclude:
    instance_groups:
    - diego-cell
  jobs:
  - name: addon-job2
    release: redis
- name: test3
  include:
    stemcell:
    - os: opensuse-42.3
    lifecycle: errand
  jobs:
  - name: addon-job3
    release: redis
stemcells:
- alias: default
  os: opensuse-42.3
  version: 28.g837c5b3-30.263-7.0.0_234.gcd7d1132
instance_groups:
- name: redis-slave
  instances: 2
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
  env:
    bosh:
      agent:
        settings:
          labels:
            custom-label: foo
          annotations:
            custom-annotation: bar
- name: diego-cell
  azs:
  - z1
  - z2
  instances: 2
  lifecycle: service
  persistent_disk: 1024
  persistent_disk_type: "standard"
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
      quarks:
        run:
          healthcheck:
            test-server:
              readiness:
                exec:
                  command:
                  - "curl --silent --fail --head http://${HOSTNAME}:8080/health"
              liveness:
                exec:
                  command:
                  - "curl --silent --fail --head http://${HOSTNAME}:8080"
        ports:
        - name: "rep-server"
          protocol: "TCP"
          internal: 1801
- name: redis-slave-errand
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
  env:
    bosh:
      agent:
        settings:
          labels:
            custom-label: foo
          annotations:
            custom-annotation: bar
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

// NatsSmall is a small manifest to start nats
const NatsSmall = `---
name: test
update:
  canary_watch_time: 20000-1200000
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

// NatsSmallWithPatch is a manifest that patches the prestart hook to loop forever
const NatsSmallWithPatch = `---
name: test
update:
  canary_watch_time: 20000
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
const Drains = `---
name: my-manifest
update:
  canary_watch_time: 20000
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

// Elaborated is a manifest with multi BOSH job specifications
const Elaborated = `name: foo-deployment
stemcells:
- alias: default
  os: opensuse-42.3
  version: 28.g837c5b3-30.263-7.0.0_234.gcd7d1132
update:
  canary_watch_time: 20000
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

// WithResources is a manifest with setting resources for a process
const WithResources = `---
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
      quarks:
        bpm:
          processes:
          - name: doppler
            requests:
              memory: 128Mi
              cpu: 5m
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
      quarks:
        bpm:
          processes:
          - name: non_existing_process
            requests:
              memory: 128Mi
              cpu: 5m
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
`

// WithProviderAndConsumer is a manifest with providers and consumers
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

// WithOverriddenBPMInfo is a manifest with overridden BPM Infos
const WithOverriddenBPMInfo = `---
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
      quarks:
        ports:
        - name: "redis"
          protocol: "TCP"
          internal: 6379
        pre_render_scripts:
          jobs:
          - |
            echo "Hello BOSH container"
        bpm:
          processes:
          - name: redis
            executable: /another/command
            limits:
              open_files: 100000
            hooks:
              pre_start: /var/vcap/jobs/pxc-mysql/bin/cleanup-socket
            env:
              # Add xtrabackup, pxc binaries, and socat to PATH
              PATH: /usr/bin:/bin:/var/vcap/packages/percona-xtrabackup/bin:/var/vcap/packages/pxc/bin:/var/vcap/packages/socat/bin
            persistent_disk: true
            ephemeral_disk: true
            additional_volumes:
            - path: /var/vcap/sys/run/pxc-mysql
              writable: true
            - path: /var/vcap/store/mysql_audit_logs
              writable: true
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

// WithAbsentBPMInfo is a manifest with an absent BPM info
const WithAbsentBPMInfo = `---
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
      quarks:
        ports:
        - name: "redis"
          protocol: "TCP"
          internal: 6379
        bpm:
          processes:
          - name: absent-process
            executable: /absent-process-command
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

// WithZeroInstances is a manifest with zero instances
const WithZeroInstances = `---
name: nats-manifest
update:
  serial: false
releases:
- name: nats
  version: "26"
  url: docker.io/cfcontainerization
  stemcell:
    os: opensuse-42.3
    version: 30.g9c91e77-30.80-7.0.0_257.gb97ced55
instance_groups:
- name: nats
  jobs:
  - name: nats
    release: nats
    properties:
      nats:
        user: admin
        password: 123456
      quarks:
        ports:
        - name: "nats1"
          protocol: "TCP"
          internal: 4222
        - name: "nats1-routes"
          protocol: TCP
          internal: 4223
`

// WithMultiBPMProcesses is a manifest with multi BPM processes
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
  update:
    canary_watch_time: 20000-1200000
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
  update:
    canary_watch_time: 20000-1200000
  instances: 3
  jobs:
  - name: fake-job-a
    release: dummy-release
  - name: fake-job-b
    release: fake-release
  - name: fake-job-c
    release: fake-release
- name: fake-ig-3
  update:
    canary_watch_time: 20000-1200000
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

// BPMRelease utilizing the test server to open two tcp ports
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
  update:
    canary_watch_time: 20000-1200000
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
const CFRouting = `
name: routing

releases:
- name: routing
  version: 0.188.0
  url: docker.io/cfcontainerization
  stemcell:
    os: opensuse-42.3
    version: 36.g03b4653-30.80-7.0.0_332.g0d8469bb
update:
  canary_watch_time: 20000-1200000
instance_groups:
- name: route_registrar
  update:
    canary_watch_time: 20000-1200000
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
const Diego = `
  name: diego

  releases:
  - name: diego
    version: 2.32.0
    url: docker.io/cfcontainerization
    stemcell:
      os: opensuse-42.3
      version: 36.g03b4653-30.80-7.0.0_332.g0d8469bb
  update:
    canary_watch_time: 20000-1200000
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

// BPMReleaseWithoutPersistentDisk doesn't contain persistent disk declaration
const BPMReleaseWithoutPersistentDisk = `
name: bpm

releases:
- name: bpm
  version: 1.0.4
  url: docker.io/cfcontainerization
  stemcell:
    os: opensuse-42.3
    version: 36.g03b4653-30.80-7.0.0_316.gcf9fe4a7
update:
  canary_watch_time: 20000-1200000
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
`

// WithMultiBPMProcessesAndPersistentDisk is a BOSH manifest with multi BPM Processes and persistent disk definition
const WithMultiBPMProcessesAndPersistentDisk = `---
name: my-manifest
update:
  canary_watch_time: 20000-1200000
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
  persistent_disk: 1024
- name: fake-ig-2
  instances: 3
  jobs:
  - name: fake-job-a
    release: dummy-release
  - name: fake-job-b
    release: fake-release
  - name: fake-job-c
    release: fake-release
  persistent_disk: 1024
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
  persistent_disk: 1024
`

// BPMReleaseWithAffinity contains affinity information
const BPMReleaseWithAffinity = `
name: bpm-affinity

releases:
- name: bpm
  version: 1.0.4
  url: docker.io/cfcontainerization
  stemcell:
    os: opensuse-42.3
    version: 36.g03b4653-30.80-7.0.0_316.gcf9fe4a7

update:
  canary_watch_time: 10000-1100000

instance_groups:
- name: bpm1
  update:
    canary_watch_time: 20000-1200000
  instances: 2
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
  env:
    bosh:
      agent:
        settings:
          affinity:
            nodeAffinity:
              requiredDuringSchedulingIgnoredDuringExecution:
                nodeSelectorTerms:
                - matchExpressions:
                  - key: beta.kubernetes.io/os
                    operator: In
                    values:
                    - linux
                    - darwin
  persistent_disk: 10
  persistent_disk_type: ((operator_test_storage_class))
- name: bpm2
  update:
    canary_watch_time: 20000-1200000
  instances: 2
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
  env:
    bosh:
      agent:
        settings:
          labels:
            instance-name: bpm2
          affinity:
            podAffinity:
              requiredDuringSchedulingIgnoredDuringExecution:
              - labelSelector:
                  matchExpressions:
                  - key: instance-name
                    operator: In
                    values:
                    - bpm2
                topologyKey: beta.kubernetes.io/os
  persistent_disk: 10
  persistent_disk_type: ((operator_test_storage_class))
- name: bpm3
  update:
    canary_watch_time: 20000-1200000
  instances: 2
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
  env:
    bosh:
      agent:
        settings:
          labels:
            instance-name: bpm3
          affinity:
            podAntiAffinity:
              preferredDuringSchedulingIgnoredDuringExecution:
                - weight: 100
                  podAffinityTerm:
                    labelSelector:
                      matchExpressions:
                      - key: instance-name
                        operator: In
                        values:
                        - bpm3
                    topologyKey: beta.kubernetes.io/os
  persistent_disk: 10
  persistent_disk_type: ((operator_test_storage_class))
`

// BOSHManifestWithTwoInstanceGroups has two instance groups nats and route_registrar
const BOSHManifestWithTwoInstanceGroups = `---
name: bosh-manifest-two-instance-groups
releases:
- name: nats
  version: "26"
  url: docker.io/cfcontainerization
  stemcell:
    os: opensuse-42.3
    version: 30.g9c91e77-30.80-7.0.0_257.gb97ced55
- name: routing
  version: 0.188.0
  url: docker.io/cfcontainerization
  stemcell:
    os: opensuse-42.3
    version: 36.g03b4653-30.80-7.0.0_332.g0d8469bb
instance_groups:
- name: nats
  update:
    canary_watch_time: 20000-1200000
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
  update:
    canary_watch_time: 20000-1200000
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
update:
  canary_watch_time: 20000-1200000
`

// ManifestWithLargeValues has large yaml values.
const ManifestWithLargeValues = `
director_uuid: ""
update:
  canary_watch_time: 20000-1200000
instance_groups:
- azs:
  - z1
  - z2
  update:
    canary_watch_time: 20000-1200000
  env:
    bosh:
      agent:
        settings: {}
      ipv6:
        enable: false
  instances: 1
  jobs:
  - name: nats
    properties:
      quarks:
        consumes: null
        debug: false
        instances: null
        is_addon: false
        ports:
        - internal: 4222
          name: nats
          protocol: TCP
        - internal: 4223
          name: nats-routes
          protocol: TCP
        pre_render_scripts: ~
        release: ""
        run:
          healthcheck:
            nats:
              liveness: null
              readiness:
                exec:
                  command:
                  - sh
                  - -c
                  - ss -nlt | grep "LISTEN.*:4222" && ss -nlt | grep "LISTEN.*:4223"
      nats:
        password: eXsUQkmHTW5ib30Re7g8QMhif90HkVZdsTfolCewO5iRlIHYsLouYNGFAJdLAeL0
        user: nats
    provides:
      nats:
        as: nats
        shared: true
    release: nats
  - name: loggregator_agent
    properties:
      quarks:
        consumes: null
        debug: false
        instances: null
        is_addon: true
        ports: null
        pre_render_scripts: ~
        release: ""
        run:
          healthcheck: null
      disable_udp: true
      grpc_port: 3459
      loggregator:
        tls:
          agent:
            cert: |
              -----BEGIN CERTIFICATE-----
              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
              -----END CERTIFICATE-----
            key: |
              -----BEGIN RSA PRIVATE KEY-----
              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
              -----END RSA PRIVATE KEY-----
          ca_cert: |
            -----BEGIN CERTIFICATE-----
            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
            -----END CERTIFICATE-----
    release: loggregator-agent
  - name: loggr-expvar-forwarder
    properties:
      log_agent:
        ca_cert: |
          -----BEGIN CERTIFICATE-----
          MIIFADCCAuigAwIBAgIUPVdppFi6U3l893jVxiW0gr760jUwDQYJKoZIhvcNAQEN
          BQAwGDEWMBQGA1UEAxMNbG9nZ3JlZ2F0b3JDQTAeFw0xOTA3MjUwNjA2MDBaFw0y
          MDA3MjQwNjA2MDBaMBgxFjAUBgNVBAMTDWxvZ2dyZWdhdG9yQ0EwggIiMA0GCSqG
          SIb3DQEBAQUAA4ICDwAwggIKAoICAQCU8UVt42KUxm38od42zhsV3O/8g3eBmUem
          7IRER844NRHlci+nnVvemFdA81bbbDsgocljVhbFnGB1ELbhNyEnqGrsk88Qou1s
          R/3wiSwg59TmLre4Kk2JbmRqzHcYJW22A4wUGspdjhchFMmstRryBCEV84IPHNH0
          aZ2SJQHsciB0mag/avvPbQ9F76uJC/eA5mG0KqH23QC1nARCmcfKrmkeXD8qFmki
          jH0nStrFVAlRX7SjNAd2N+64uVzisGO0lze+V8o7MAr7pJxzmPfGs0QYhFpFHgcO
          rOEvNW1HTanc8an338DDlZSSqdVqdBhRXXFSP75+D0y8UNajVxXzUvOJ3rZfNbFV
          LlnOTHW/ItiOJjzodUfhE3jzjv4DqvKIk/Mrp0HVpgH5niGWgF4LIAav7cK7fVgd
          xACtuUAhAsL3RFddvz8sY4ixm8O0jvAUerCRPnjnA+Uj/1i7XX9cjmIVfcxwjcfH
          mLFSnXtX6+w4m4tWEIN/BptwLdfnMB2DzRXbDQE7m+vxITfBLaY/vK5NA8lil/n8
          FISPtLczIORvjkRrwPKLv435EUxd0EIJFVj7wKaWZDPmtIwOHex1n12BTzlfToig
          FrJi/KwwF4+GwnfERkJkd6JafB7/28Gqp6+UzXcKphBOjGDhaAu7/NlOteRsRLHs
          M0DxqcMh3QIDAQABo0IwQDAOBgNVHQ8BAf8EBAMCAQYwDwYDVR0TAQH/BAUwAwEB
          /zAdBgNVHQ4EFgQUEC7E2EDXK17QW3D7QxGeB0pUudgwDQYJKoZIhvcNAQENBQAD
          ggIBAGUcEEk1dKdn73IapvFhrDKHNYSLEGgIVpyvnwjwi4EyXzHNhmGMnHJkAYRg
          KaWBfao8ngYawfEtFpvz1pdpOW+Ul8bMtcC+mJlxI/E/Od0WWNE6QRNdWsoH5JSj
          ef+SepxE6ztMfzayC4Tmp85vT1TWi7/2maHuefosAKiwovtcsnr54Y6GJkozY2Hd
          46V185MuDK14BeS9Yne9XWSDOdjZH20kRHtoRbxRz15krFmbbpIyek2mss2nVV2d
          t1pUK4er6R4y3QHBn7QBq5kAxiKhFY6yA88+uhX2jf4u5uroG0CHGdZmKlGrb4N/
          fC/1BSBo16V6EOZAy35ktlg4oSCbeJmDXYwZzVvOpQGPRqB7lfDM1bZcv8vdxrXn
          YALcq7OVkRFeCy9HDEvwARfQ1axTZM+tKrcQav7dIKNGr4inzg9tNBhtORlZudhi
          AfpHyEr6rMFk8t63Q45MXMp5L9x4ThyPjyfo17BwhfjY47ibbHvo4vy9O/vbcw4i
          NASFM8VUwtFO9Ip3GAVtUZR4V+i77SsDo3B8546T/KDP2cBjnP+sSjUvtpAGLDFJ
          Ha4RWJN4IE+DdVIcipKT2yCzI3Xr8NUO+Q+h7wVgtE8e2sNrsM5X76ILtZBlOfPy
          jVdYnn9gIxqS6iWHiGfAHf4Bs+shXicXye88TfeNDnHvLw/Q
          -----END CERTIFICATE-----
        client_cert: |
          -----BEGIN CERTIFICATE-----
          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
          03oz0k9Iq+xsp2UEzLV0TCPPEg==
          -----END CERTIFICATE-----
        client_key: |
          -----BEGIN RSA PRIVATE KEY-----
          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
          -----END RSA PRIVATE KEY-----
    release: loggregator-agent
  - name: loggr-forwarder-agent
    properties:
      quarks:
        consumes: null
        debug: false
        instances: null
        is_addon: true
        ports: null
        pre_render_scripts: ~
        release: ""
        run:
          healthcheck: null
      tls:
        ca_cert: |
          -----BEGIN CERTIFICATE-----
          MIIFADCCAuigAwIBAgIUPVdppFi6U3l893jVxiW0gr760jUwDQYJKoZIhvcNAQEN
          BQAwGDEWMBQGA1UEAxMNbG9nZ3JlZ2F0b3JDQTAeFw0xOTA3MjUwNjA2MDBaFw0y
          MDA3MjQwNjA2MDBaMBgxFjAUBgNVBAMTDWxvZ2dyZWdhdG9yQ0EwggIiMA0GCSqG
          SIb3DQEBAQUAA4ICDwAwggIKAoICAQCU8UVt42KUxm38od42zhsV3O/8g3eBmUem
          7IRER844NRHlci+nnVvemFdA81bbbDsgocljVhbFnGB1ELbhNyEnqGrsk88Qou1s
          R/3wiSwg59TmLre4Kk2JbmRqzHcYJW22A4wUGspdjhchFMmstRryBCEV84IPHNH0
          aZ2SJQHsciB0mag/avvPbQ9F76uJC/eA5mG0KqH23QC1nARCmcfKrmkeXD8qFmki
          jH0nStrFVAlRX7SjNAd2N+64uVzisGO0lze+V8o7MAr7pJxzmPfGs0QYhFpFHgcO
          rOEvNW1HTanc8an338DDlZSSqdVqdBhRXXFSP75+D0y8UNajVxXzUvOJ3rZfNbFV
          LlnOTHW/ItiOJjzodUfhE3jzjv4DqvKIk/Mrp0HVpgH5niGWgF4LIAav7cK7fVgd
          xACtuUAhAsL3RFddvz8sY4ixm8O0jvAUerCRPnjnA+Uj/1i7XX9cjmIVfcxwjcfH
          mLFSnXtX6+w4m4tWEIN/BptwLdfnMB2DzRXbDQE7m+vxITfBLaY/vK5NA8lil/n8
          FISPtLczIORvjkRrwPKLv435EUxd0EIJFVj7wKaWZDPmtIwOHex1n12BTzlfToig
          FrJi/KwwF4+GwnfERkJkd6JafB7/28Gqp6+UzXcKphBOjGDhaAu7/NlOteRsRLHs
          M0DxqcMh3QIDAQABo0IwQDAOBgNVHQ8BAf8EBAMCAQYwDwYDVR0TAQH/BAUwAwEB
          /zAdBgNVHQ4EFgQUEC7E2EDXK17QW3D7QxGeB0pUudgwDQYJKoZIhvcNAQENBQAD
          ggIBAGUcEEk1dKdn73IapvFhrDKHNYSLEGgIVpyvnwjwi4EyXzHNhmGMnHJkAYRg
          KaWBfao8ngYawfEtFpvz1pdpOW+Ul8bMtcC+mJlxI/E/Od0WWNE6QRNdWsoH5JSj
          ef+SepxE6ztMfzayC4Tmp85vT1TWi7/2maHuefosAKiwovtcsnr54Y6GJkozY2Hd
          46V185MuDK14BeS9Yne9XWSDOdjZH20kRHtoRbxRz15krFmbbpIyek2mss2nVV2d
          t1pUK4er6R4y3QHBn7QBq5kAxiKhFY6yA88+uhX2jf4u5uroG0CHGdZmKlGrb4N/
          fC/1BSBo16V6EOZAy35ktlg4oSCbeJmDXYwZzVvOpQGPRqB7lfDM1bZcv8vdxrXn
          YALcq7OVkRFeCy9HDEvwARfQ1axTZM+tKrcQav7dIKNGr4inzg9tNBhtORlZudhi
          AfpHyEr6rMFk8t63Q45MXMp5L9x4ThyPjyfo17BwhfjY47ibbHvo4vy9O/vbcw4i
          NASFM8VUwtFO9Ip3GAVtUZR4V+i77SsDo3B8546T/KDP2cBjnP+sSjUvtpAGLDFJ
          Ha4RWJN4IE+DdVIcipKT2yCzI3Xr8NUO+Q+h7wVgtE8e2sNrsM5X76ILtZBlOfPy
          jVdYnn9gIxqS6iWHiGfAHf4Bs+shXicXye88TfeNDnHvLw/Q
          -----END CERTIFICATE-----
        cert: |
          -----BEGIN CERTIFICATE-----
          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
          -----END CERTIFICATE-----
        key: |
          -----BEGIN RSA PRIVATE KEY-----
          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
          -----END RSA PRIVATE KEY-----
    release: loggregator-agent
  name: nats
  networks:
  - name: default
  stemcell: default
  vm_resources: null
  vm_type: minimal
- azs:
  - z1
  - z2
  env:
    bosh:
      agent:
        settings: {}
      ipv6:
        enable: false
  instances: 1
  jobs:
  - name: adapter
    properties:
      scalablesyslog:
        adapter:
          bosh_dns: false
          logs:
            addr: scf-dev-log-api:8082
          tls:
            ca: |
              -----BEGIN CERTIFICATE-----
              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
              -----END CERTIFICATE-----
            cert: |
              -----BEGIN CERTIFICATE-----
              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
              dbnVI+Oh3w==
              -----END CERTIFICATE-----
            cn: ss-adapter
            key: |
              -----BEGIN RSA PRIVATE KEY-----
              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
              -----END RSA PRIVATE KEY-----
        adapter_rlp:
          tls:
            ca: |
              -----BEGIN CERTIFICATE-----
              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
              -----END CERTIFICATE-----
            cert: |
              -----BEGIN CERTIFICATE-----
              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
              gQLD4XaONjUaiXV50NuD
              -----END CERTIFICATE-----
            cn: reverselogproxy
            key: |
              -----BEGIN RSA PRIVATE KEY-----
              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
              -----END RSA PRIVATE KEY-----
    release: cf-syslog-drain
  - name: loggregator_agent
    properties:
      loggregator:
        tls:
          agent:
            cert: |
              -----BEGIN CERTIFICATE-----
              MIIFOzCCAyOgAwIBAgIUUkyFTjBNZJyp4xFMwq9vImhV/OUwDQYJKoZIhvcNAQEN
              BQAwGDEWMBQGA1UEAxMNbG9nZ3JlZ2F0b3JDQTAeFw0xOTA3MjUwNjA2MDBaFw0y
              MDA3MjQwNjA2MDBaMBExDzANBgNVBAMTBm1ldHJvbjCCAiIwDQYJKoZIhvcNAQEB
              BQADggIPADCCAgoCggIBANRWsOAZNcCZghQsXrQKaHOpdgibljN5K0ZeCXwsKbOa
              XoM8aNB5I+XxHFLYkB6zm5cXv8n6UHeiFaemxjSMT7shO/yTyYq6MpfSdHM1Eops
              LOrKCqXDwi+hxvQmTKxtmVb/Ja6RqnsVDaIkLL/DN803De8yEwPexxYWHMIKwSaY
              WaVYgZugp89HGzcoeX+N2WXmPOrqMi2OZ1ZC0+lUpUjC0EJYBn+oYF234VQSsCIi
              h++AAFbgnzBV4xl8/NeGP1Xqqu57qlz3tFyFoj+k8iFa6Buz5Dv1+JAt+8MERplY
              nIDlHEfmD5TI9cPVDHnBp7Gth+Fv4s5RcnFLOUR+xWvIJ9XiqJUXtFaN0sTIC/DV
              Iocg92NQDOLsCRNJV47jV4c1biMvV0AICZdlMebRRJRAgfd3Um4CriOnvYNsoFuC
              ee10BeyiP1FPJz6dUeTXRgDq9aYlZf59Q63b0zaT1IYK0eHmTzlKduLn04dL5p/T
              vJIR6nSaHKdi6/XTKDnT3KuuDb/rYPPTHGprFW0czt/w0u3CSJFnoH5r9kbVZn7j
              4xMZoY3JPz8nzPU9tW6pNenc/vMWp5DYe2IlyiwkbUM5xAPKO9DxSxnn/aussuyB
              KJErotN20YGOZcGVskc5DwqrntWZFL1pFQf1IgcBzCjM6TomDHkp5Jn6Lqvad9xH
              AgMBAAGjgYMwgYAwHQYDVR0lBBYwFAYIKwYBBQUHAwEGCCsGAQUFBwMCMAwGA1Ud
              EwEB/wQCMAAwHQYDVR0OBBYEFPHm52ztGFbCDLj65PEj81S058jNMB8GA1UdIwQY
              MBaAFBAuxNhA1yte0Ftw+0MRngdKVLnYMBEGA1UdEQQKMAiCBm1ldHJvbjANBgkq
              hkiG9w0BAQ0FAAOCAgEAFIVP3POc1jf9mhTD2o9A2u+pm+LL8VBjPeA7X0PkFT3R
              VwG5CbAQqmY9giNBCV00RruYNE1qrlsM06kQnHbglqAIlEMFz50M9yzyYvKxw4uQ
              FSnSdEdl1rgF0G82Q2IA0jFCxZ8sz/GzGROBHbNv5FQs7leNYmykvUKkLJdwBskn
              CsZ7PA1V9mKMogD3BbqH3lB7nRwRmA1LMOSu50l6PJAH+gdTnVzV2QF6B9shJ+dT
              TSzsL2GSjoAv0/F1jAVUbmroNyoZ7/KoAecRRedzGnpWDrRUsvktlGOhGpjd9f3S
              QWIn0KjvOiJVUygXBbvgJ8X5bGTyUgxKa02N4OaMHT18hPVjyhD5nzgq/hGrbjvf
              tFSEwgKan2080XjOeVubFhxcMVTp3gD6Q0EAsTuxaw1SYkbqXxb6rRBeIWkMavN/
              cRsgaLj16uNKXxHHRRQm0BV029udogqOQVqDwOlMDFFFSQmMgx1kWzcU4leyiaZT
              frmOKKy0K6czUQ/tE4Bt9/7SLPIysMCDSxE4sPefS+m030LpaVgGidiEmc/Fs9pW
              /15rKzOePCVXG7IBzkNJmb0SRdCrG8sPn56O5Gc5EiULZJL24FJzRysToxf7RhFz
              2tZ5jxFlhSjRZLTxXAJirEcjAgzrpX+47D/UuWcQiuNdbSZk4MZuCFEbYVho9C8=
              -----END CERTIFICATE-----
            key: |
              -----BEGIN RSA PRIVATE KEY-----
              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
              -----END RSA PRIVATE KEY-----
          ca_cert: |
            -----BEGIN CERTIFICATE-----
            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
            -----END CERTIFICATE-----
    release: loggregator-agent
  - name: loggr-expvar-forwarder
    properties:
      log_agent:
        ca_cert: |
          -----BEGIN CERTIFICATE-----
          MIIFADCCAuigAwIBAgIUPVdppFi6U3l893jVxiW0gr760jUwDQYJKoZIhvcNAQEN
          BQAwGDEWMBQGA1UEAxMNbG9nZ3JlZ2F0b3JDQTAeFw0xOTA3MjUwNjA2MDBaFw0y
          MDA3MjQwNjA2MDBaMBgxFjAUBgNVBAMTDWxvZ2dyZWdhdG9yQ0EwggIiMA0GCSqG
          SIb3DQEBAQUAA4ICDwAwggIKAoICAQCU8UVt42KUxm38od42zhsV3O/8g3eBmUem
          7IRER844NRHlci+nnVvemFdA81bbbDsgocljVhbFnGB1ELbhNyEnqGrsk88Qou1s
          R/3wiSwg59TmLre4Kk2JbmRqzHcYJW22A4wUGspdjhchFMmstRryBCEV84IPHNH0
          aZ2SJQHsciB0mag/avvPbQ9F76uJC/eA5mG0KqH23QC1nARCmcfKrmkeXD8qFmki
          jH0nStrFVAlRX7SjNAd2N+64uVzisGO0lze+V8o7MAr7pJxzmPfGs0QYhFpFHgcO
          rOEvNW1HTanc8an338DDlZSSqdVqdBhRXXFSP75+D0y8UNajVxXzUvOJ3rZfNbFV
          LlnOTHW/ItiOJjzodUfhE3jzjv4DqvKIk/Mrp0HVpgH5niGWgF4LIAav7cK7fVgd
          xACtuUAhAsL3RFddvz8sY4ixm8O0jvAUerCRPnjnA+Uj/1i7XX9cjmIVfcxwjcfH
          mLFSnXtX6+w4m4tWEIN/BptwLdfnMB2DzRXbDQE7m+vxITfBLaY/vK5NA8lil/n8
          FISPtLczIORvjkRrwPKLv435EUxd0EIJFVj7wKaWZDPmtIwOHex1n12BTzlfToig
          FrJi/KwwF4+GwnfERkJkd6JafB7/28Gqp6+UzXcKphBOjGDhaAu7/NlOteRsRLHs
          M0DxqcMh3QIDAQABo0IwQDAOBgNVHQ8BAf8EBAMCAQYwDwYDVR0TAQH/BAUwAwEB
          /zAdBgNVHQ4EFgQUEC7E2EDXK17QW3D7QxGeB0pUudgwDQYJKoZIhvcNAQENBQAD
          ggIBAGUcEEk1dKdn73IapvFhrDKHNYSLEGgIVpyvnwjwi4EyXzHNhmGMnHJkAYRg
          KaWBfao8ngYawfEtFpvz1pdpOW+Ul8bMtcC+mJlxI/E/Od0WWNE6QRNdWsoH5JSj
          ef+SepxE6ztMfzayC4Tmp85vT1TWi7/2maHuefosAKiwovtcsnr54Y6GJkozY2Hd
          46V185MuDK14BeS9Yne9XWSDOdjZH20kRHtoRbxRz15krFmbbpIyek2mss2nVV2d
          t1pUK4er6R4y3QHBn7QBq5kAxiKhFY6yA88+uhX2jf4u5uroG0CHGdZmKlGrb4N/
          fC/1BSBo16V6EOZAy35ktlg4oSCbeJmDXYwZzVvOpQGPRqB7lfDM1bZcv8vdxrXn
          YALcq7OVkRFeCy9HDEvwARfQ1axTZM+tKrcQav7dIKNGr4inzg9tNBhtORlZudhi
          AfpHyEr6rMFk8t63Q45MXMp5L9x4ThyPjyfo17BwhfjY47ibbHvo4vy9O/vbcw4i
          NASFM8VUwtFO9Ip3GAVtUZR4V+i77SsDo3B8546T/KDP2cBjnP+sSjUvtpAGLDFJ
          Ha4RWJN4IE+DdVIcipKT2yCzI3Xr8NUO+Q+h7wVgtE8e2sNrsM5X76ILtZBlOfPy
          jVdYnn9gIxqS6iWHiGfAHf4Bs+shXicXye88TfeNDnHvLw/Q
          -----END CERTIFICATE-----
        client_cert: |
          -----BEGIN CERTIFICATE-----
          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
          03oz0k9Iq+xsp2UEzLV0TCPPEg==
          -----END CERTIFICATE-----
        client_key: |
          -----BEGIN RSA PRIVATE KEY-----
          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
          -----END RSA PRIVATE KEY-----
    release: loggregator-agent
  - name: loggr-forwarder-agent
    properties:
      quarks:
        consumes: null
        debug: false
        instances: null
        is_addon: true
        ports: null
        pre_render_scripts: ~
        release: ""
        run:
          healthcheck: null
      tls:
        ca_cert: |
          -----BEGIN CERTIFICATE-----
          MIIFADCCAuigAwIBAgIUPVdppFi6U3l893jVxiW0gr760jUwDQYJKoZIhvcNAQEN
          BQAwGDEWMBQGA1UEAxMNbG9nZ3JlZ2F0b3JDQTAeFw0xOTA3MjUwNjA2MDBaFw0y
          MDA3MjQwNjA2MDBaMBgxFjAUBgNVBAMTDWxvZ2dyZWdhdG9yQ0EwggIiMA0GCSqG
          SIb3DQEBAQUAA4ICDwAwggIKAoICAQCU8UVt42KUxm38od42zhsV3O/8g3eBmUem
          7IRER844NRHlci+nnVvemFdA81bbbDsgocljVhbFnGB1ELbhNyEnqGrsk88Qou1s
          R/3wiSwg59TmLre4Kk2JbmRqzHcYJW22A4wUGspdjhchFMmstRryBCEV84IPHNH0
          aZ2SJQHsciB0mag/avvPbQ9F76uJC/eA5mG0KqH23QC1nARCmcfKrmkeXD8qFmki
          jH0nStrFVAlRX7SjNAd2N+64uVzisGO0lze+V8o7MAr7pJxzmPfGs0QYhFpFHgcO
          rOEvNW1HTanc8an338DDlZSSqdVqdBhRXXFSP75+D0y8UNajVxXzUvOJ3rZfNbFV
          LlnOTHW/ItiOJjzodUfhE3jzjv4DqvKIk/Mrp0HVpgH5niGWgF4LIAav7cK7fVgd
          xACtuUAhAsL3RFddvz8sY4ixm8O0jvAUerCRPnjnA+Uj/1i7XX9cjmIVfcxwjcfH
          mLFSnXtX6+w4m4tWEIN/BptwLdfnMB2DzRXbDQE7m+vxITfBLaY/vK5NA8lil/n8
          FISPtLczIORvjkRrwPKLv435EUxd0EIJFVj7wKaWZDPmtIwOHex1n12BTzlfToig
          FrJi/KwwF4+GwnfERkJkd6JafB7/28Gqp6+UzXcKphBOjGDhaAu7/NlOteRsRLHs
          M0DxqcMh3QIDAQABo0IwQDAOBgNVHQ8BAf8EBAMCAQYwDwYDVR0TAQH/BAUwAwEB
          /zAdBgNVHQ4EFgQUEC7E2EDXK17QW3D7QxGeB0pUudgwDQYJKoZIhvcNAQENBQAD
          ggIBAGUcEEk1dKdn73IapvFhrDKHNYSLEGgIVpyvnwjwi4EyXzHNhmGMnHJkAYRg
          KaWBfao8ngYawfEtFpvz1pdpOW+Ul8bMtcC+mJlxI/E/Od0WWNE6QRNdWsoH5JSj
          ef+SepxE6ztMfzayC4Tmp85vT1TWi7/2maHuefosAKiwovtcsnr54Y6GJkozY2Hd
          46V185MuDK14BeS9Yne9XWSDOdjZH20kRHtoRbxRz15krFmbbpIyek2mss2nVV2d
          t1pUK4er6R4y3QHBn7QBq5kAxiKhFY6yA88+uhX2jf4u5uroG0CHGdZmKlGrb4N/
          fC/1BSBo16V6EOZAy35ktlg4oSCbeJmDXYwZzVvOpQGPRqB7lfDM1bZcv8vdxrXn
          YALcq7OVkRFeCy9HDEvwARfQ1axTZM+tKrcQav7dIKNGr4inzg9tNBhtORlZudhi
          AfpHyEr6rMFk8t63Q45MXMp5L9x4ThyPjyfo17BwhfjY47ibbHvo4vy9O/vbcw4i
          NASFM8VUwtFO9Ip3GAVtUZR4V+i77SsDo3B8546T/KDP2cBjnP+sSjUvtpAGLDFJ
          Ha4RWJN4IE+DdVIcipKT2yCzI3Xr8NUO+Q+h7wVgtE8e2sNrsM5X76ILtZBlOfPy
          jVdYnn9gIxqS6iWHiGfAHf4Bs+shXicXye88TfeNDnHvLw/Q
          -----END CERTIFICATE-----
        cert: |
          -----BEGIN CERTIFICATE-----
          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
          -----END CERTIFICATE-----
        key: |
          -----BEGIN RSA PRIVATE KEY-----
          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
          -----END RSA PRIVATE KEY-----
    release: loggregator-agent
  name: adapter
  networks:
  - name: default
  stemcell: default
  vm_resources: null
  vm_type: minimal
- azs:
  - z1
  env:
    bosh:
      agent:
        settings: {}
      ipv6:
        enable: false
  instances: 1
  jobs:
  - name: mysql
    properties:
      quarks:
        bpm:
          processes:
          - args:
            - -c
            - |-
              wait_for_file() {
                local file_path="$1"
                local timeout="${2:-30}"
                until [[ -f "${file_path}" ]] || [[ "$timeout" == "0" ]]; do sleep 1; timeout=$(expr $timeout - 1); done
                if [[ "${timeout}" == 0 ]]; then return 1; fi
                return 0
              }

              /var/vcap/jobs/mysql/bin/mariadb_ctl start

              pid_file="/var/vcap/sys/run/mysql/mysql.pid"
              log_file="/var/vcap/sys/log/mysql/mariadb_ctrl.combined.log"

              wait_for_file "${pid_file}" || {
                echo "${pid_file} did not get created"
                exit 1
              }

              wait_for_file "${log_file}" || {
                echo "${log_file} did not get created"
                exit 1
              }

              tail \
                --pid $(cat "${pid_file}") \
                --follow "${log_file}"
            executable: /bin/bash
            hooks: {}
            limits:
              open_files: 1048576
            name: mariadb_ctrl
            persistent_disk: true
            unsafe: {}
          unsupported_template: false
        consumes: null
        debug: false
        instances: null
        is_addon: false
        ports:
        - internal: 3306
          name: mysql
          protocol: TCP
        pre_render_scripts:
          bpm:
          - |-
            #!/usr/bin/env bash

            set -o errexit

            # Patch pre-start-setup.erb to play nice with BPM's persistent disk. Instead of checking for the
            # existence of the directory /var/vcap/store/mysql, it checks for the existence of the file
            # /var/vcap/store/mysql/setup_succeeded, which is also created in a command from this patch.
            patch /var/vcap/all-releases/jobs-src/cf-mysql/mysql/templates/pre-start-setup.erb <<'EOT'
            82,84c82,84
            < if ! test -d ${datadir}; then
            <   log "pre-start setup script: making ${datadir} and running /var/vcap/packages/mariadb/scripts/mysql_install_db"
            <   mkdir -p ${datadir}
            ---
            > setup_control_file="${datadir}/setup_succeeded"
            > if ! test -e "${setup_control_file}"; then
            >   log "pre-start setup script: running /var/vcap/packages/mariadb/scripts/mysql_install_db"
            89a90
            >   touch "${setup_control_file}"
            EOT
        release: ""
        run:
          healthcheck: null
      cf_mysql:
        mysql:
          admin_password: xfnEWZPrzkgpgTA6MwHTfGEjOIhnoFvRZebUx0n4i9759hM6hKGvtrd3T1FmwV4j
          binlog_enabled: false
          cluster_health:
            password: S7tKA6AgLpL20nWogEgA5wKXojYRZWmxkMRwq1CMHOpus22sE7guRhNAc30mEfIx
          enable_galera: false
          galera_healthcheck:
            db_password: a48N3rz63UCzwTqaV7vlowxhwpg26PNZxd1GjguZvwjXXMaRoWBBctq3ILOobpWs
            endpoint_password: 6p8jmrn00uCVZSL9HXBPALfJwHJIArtTT9qW9b1CNNHM9bWfqnqIEkYOoh4TtCnc
            endpoint_username: galera_healthcheck
          port: 3306
          seeded_databases:
          - name: cloud_controller
            password: YcNnnukcHIL6kvPZeWD6O8rN027vzJa7bP6gNH73WeaZItMkDcZDfIdefzAdH1gR
            username: cloud_controller
          - name: credhub
            password: SS1qvVc805kJDlqDi7kVdCEs66QEAMYWn0RclNeoHLgSKqfrxmS1pQQJ1rxH6EHe
            username: credhub
          - name: diego
            password: PpsySGsiP96ZhoP4j9ihSc5qADV4DiVOHq83bmUBTs2EQCe3DHxTegN77fgvFhC4
            username: diego
          - name: network_connectivity
            password: zwmy022jTL4Njm7pJlYzQdNgh0f96kp6SXL4B6PL0YL2qfy6njHZ9IYQE57t7HrS
            username: network_connectivity
          - name: network_policy
            password: Yl7iIUa3ptOMqrnDarkpy00u32diyiZPjxtPTYsjBRsfOU4i53PrzdZONrbSAc0i
            username: network_policy
          - name: routing-api
            password: y9B7Bqw5h9EfwFGwsRbWguZxLLyPCrsITbnJ2PMPKbcvu02PDi099uSw3B4JZCis
            username: routing-api
          - name: uaa
            password: vPpRI4J96Nq85fAqpL6sIReMxJqXJlTB9fpmvT34DO47n1oNk061MJAU0s9gPmb8
            username: uaa
          - name: locket
            password: r37BtzZuHU3CcSKhjIcAVvXc7oNdBB6NJdETwnJwUab7HkRPgU3pWequbWi7vEAv
            username: locket
          tls:
            ca_certificate: |
              -----BEGIN CERTIFICATE-----
              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
              -----END CERTIFICATE-----
            server_certificate: |
              -----BEGIN CERTIFICATE-----
              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
              XlM+5CA03NWuRBgzAtAnxnLnfA==
              -----END CERTIFICATE-----
            server_key: |
              -----BEGIN RSA PRIVATE KEY-----
              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
              -----END RSA PRIVATE KEY-----
    release: cf-mysql
  - name: loggregator_agent
    properties:
      loggregator:
        tls:
          agent:
            cert: |
              -----BEGIN CERTIFICATE-----
              MIIFOzCCAyOgAwIBAgIUUkyFTjBNZJyp4xFMwq9vImhV/OUwDQYJKoZIhvcNAQEN
              BQAwGDEWMBQGA1UEAxMNbG9nZ3JlZ2F0b3JDQTAeFw0xOTA3MjUwNjA2MDBaFw0y
              MDA3MjQwNjA2MDBaMBExDzANBgNVBAMTBm1ldHJvbjCCAiIwDQYJKoZIhvcNAQEB
              BQADggIPADCCAgoCggIBANRWsOAZNcCZghQsXrQKaHOpdgibljN5K0ZeCXwsKbOa
              XoM8aNB5I+XxHFLYkB6zm5cXv8n6UHeiFaemxjSMT7shO/yTyYq6MpfSdHM1Eops
              LOrKCqXDwi+hxvQmTKxtmVb/Ja6RqnsVDaIkLL/DN803De8yEwPexxYWHMIKwSaY
              WaVYgZugp89HGzcoeX+N2WXmPOrqMi2OZ1ZC0+lUpUjC0EJYBn+oYF234VQSsCIi
              h++AAFbgnzBV4xl8/NeGP1Xqqu57qlz3tFyFoj+k8iFa6Buz5Dv1+JAt+8MERplY
              nIDlHEfmD5TI9cPVDHnBp7Gth+Fv4s5RcnFLOUR+xWvIJ9XiqJUXtFaN0sTIC/DV
              Iocg92NQDOLsCRNJV47jV4c1biMvV0AICZdlMebRRJRAgfd3Um4CriOnvYNsoFuC
              ee10BeyiP1FPJz6dUeTXRgDq9aYlZf59Q63b0zaT1IYK0eHmTzlKduLn04dL5p/T
              vJIR6nSaHKdi6/XTKDnT3KuuDb/rYPPTHGprFW0czt/w0u3CSJFnoH5r9kbVZn7j
              4xMZoY3JPz8nzPU9tW6pNenc/vMWp5DYe2IlyiwkbUM5xAPKO9DxSxnn/aussuyB
              KJErotN20YGOZcGVskc5DwqrntWZFL1pFQf1IgcBzCjM6TomDHkp5Jn6Lqvad9xH
              AgMBAAGjgYMwgYAwHQYDVR0lBBYwFAYIKwYBBQUHAwEGCCsGAQUFBwMCMAwGA1Ud
              EwEB/wQCMAAwHQYDVR0OBBYEFPHm52ztGFbCDLj65PEj81S058jNMB8GA1UdIwQY
              MBaAFBAuxNhA1yte0Ftw+0MRngdKVLnYMBEGA1UdEQQKMAiCBm1ldHJvbjANBgkq
              hkiG9w0BAQ0FAAOCAgEAFIVP3POc1jf9mhTD2o9A2u+pm+LL8VBjPeA7X0PkFT3R
              VwG5CbAQqmY9giNBCV00RruYNE1qrlsM06kQnHbglqAIlEMFz50M9yzyYvKxw4uQ
              FSnSdEdl1rgF0G82Q2IA0jFCxZ8sz/GzGROBHbNv5FQs7leNYmykvUKkLJdwBskn
              CsZ7PA1V9mKMogD3BbqH3lB7nRwRmA1LMOSu50l6PJAH+gdTnVzV2QF6B9shJ+dT
              TSzsL2GSjoAv0/F1jAVUbmroNyoZ7/KoAecRRedzGnpWDrRUsvktlGOhGpjd9f3S
              QWIn0KjvOiJVUygXBbvgJ8X5bGTyUgxKa02N4OaMHT18hPVjyhD5nzgq/hGrbjvf
              tFSEwgKan2080XjOeVubFhxcMVTp3gD6Q0EAsTuxaw1SYkbqXxb6rRBeIWkMavN/
              cRsgaLj16uNKXxHHRRQm0BV029udogqOQVqDwOlMDFFFSQmMgx1kWzcU4leyiaZT
              frmOKKy0K6czUQ/tE4Bt9/7SLPIysMCDSxE4sPefS+m030LpaVgGidiEmc/Fs9pW
              /15rKzOePCVXG7IBzkNJmb0SRdCrG8sPn56O5Gc5EiULZJL24FJzRysToxf7RhFz
              2tZ5jxFlhSjRZLTxXAJirEcjAgzrpX+47D/UuWcQiuNdbSZk4MZuCFEbYVho9C8=
              -----END CERTIFICATE-----
            key: |
              -----BEGIN RSA PRIVATE KEY-----
              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
              -----END RSA PRIVATE KEY-----
          ca_cert: |
            -----BEGIN CERTIFICATE-----
            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
            -----END CERTIFICATE-----
    release: loggregator-agent
  - name: loggr-forwarder-agent
    properties:
      quarks:
        consumes: null
        debug: false
        instances: null
        is_addon: true
        ports: null
        pre_render_scripts: ~
        release: ""
        run:
          healthcheck: null
      tls:
        ca_cert: |
          -----BEGIN CERTIFICATE-----
          MIIFADCCAuigAwIBAgIUPVdppFi6U3l893jVxiW0gr760jUwDQYJKoZIhvcNAQEN
          BQAwGDEWMBQGA1UEAxMNbG9nZ3JlZ2F0b3JDQTAeFw0xOTA3MjUwNjA2MDBaFw0y
          MDA3MjQwNjA2MDBaMBgxFjAUBgNVBAMTDWxvZ2dyZWdhdG9yQ0EwggIiMA0GCSqG
          SIb3DQEBAQUAA4ICDwAwggIKAoICAQCU8UVt42KUxm38od42zhsV3O/8g3eBmUem
          7IRER844NRHlci+nnVvemFdA81bbbDsgocljVhbFnGB1ELbhNyEnqGrsk88Qou1s
          R/3wiSwg59TmLre4Kk2JbmRqzHcYJW22A4wUGspdjhchFMmstRryBCEV84IPHNH0
          aZ2SJQHsciB0mag/avvPbQ9F76uJC/eA5mG0KqH23QC1nARCmcfKrmkeXD8qFmki
          jH0nStrFVAlRX7SjNAd2N+64uVzisGO0lze+V8o7MAr7pJxzmPfGs0QYhFpFHgcO
          rOEvNW1HTanc8an338DDlZSSqdVqdBhRXXFSP75+D0y8UNajVxXzUvOJ3rZfNbFV
          LlnOTHW/ItiOJjzodUfhE3jzjv4DqvKIk/Mrp0HVpgH5niGWgF4LIAav7cK7fVgd
          xACtuUAhAsL3RFddvz8sY4ixm8O0jvAUerCRPnjnA+Uj/1i7XX9cjmIVfcxwjcfH
          mLFSnXtX6+w4m4tWEIN/BptwLdfnMB2DzRXbDQE7m+vxITfBLaY/vK5NA8lil/n8
          FISPtLczIORvjkRrwPKLv435EUxd0EIJFVj7wKaWZDPmtIwOHex1n12BTzlfToig
          FrJi/KwwF4+GwnfERkJkd6JafB7/28Gqp6+UzXcKphBOjGDhaAu7/NlOteRsRLHs
          M0DxqcMh3QIDAQABo0IwQDAOBgNVHQ8BAf8EBAMCAQYwDwYDVR0TAQH/BAUwAwEB
          /zAdBgNVHQ4EFgQUEC7E2EDXK17QW3D7QxGeB0pUudgwDQYJKoZIhvcNAQENBQAD
          ggIBAGUcEEk1dKdn73IapvFhrDKHNYSLEGgIVpyvnwjwi4EyXzHNhmGMnHJkAYRg
          KaWBfao8ngYawfEtFpvz1pdpOW+Ul8bMtcC+mJlxI/E/Od0WWNE6QRNdWsoH5JSj
          ef+SepxE6ztMfzayC4Tmp85vT1TWi7/2maHuefosAKiwovtcsnr54Y6GJkozY2Hd
          46V185MuDK14BeS9Yne9XWSDOdjZH20kRHtoRbxRz15krFmbbpIyek2mss2nVV2d
          t1pUK4er6R4y3QHBn7QBq5kAxiKhFY6yA88+uhX2jf4u5uroG0CHGdZmKlGrb4N/
          fC/1BSBo16V6EOZAy35ktlg4oSCbeJmDXYwZzVvOpQGPRqB7lfDM1bZcv8vdxrXn
          YALcq7OVkRFeCy9HDEvwARfQ1axTZM+tKrcQav7dIKNGr4inzg9tNBhtORlZudhi
          AfpHyEr6rMFk8t63Q45MXMp5L9x4ThyPjyfo17BwhfjY47ibbHvo4vy9O/vbcw4i
          NASFM8VUwtFO9Ip3GAVtUZR4V+i77SsDo3B8546T/KDP2cBjnP+sSjUvtpAGLDFJ
          Ha4RWJN4IE+DdVIcipKT2yCzI3Xr8NUO+Q+h7wVgtE8e2sNrsM5X76ILtZBlOfPy
          jVdYnn9gIxqS6iWHiGfAHf4Bs+shXicXye88TfeNDnHvLw/Q
          -----END CERTIFICATE-----
        cert: |
          -----BEGIN CERTIFICATE-----
          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
          -----END CERTIFICATE-----
        key: |
          -----BEGIN RSA PRIVATE KEY-----
          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
          -----END RSA PRIVATE KEY-----
    release: loggregator-agent
  migrated_from:
  - name: mysql
  - name: singleton-database
  name: database
  networks:
  - name: default
  persistent_disk: 20480
  stemcell: default
  update:
    canaries: 0
    canary_watch_time: ""
    max_in_flight: ""
    serial: true
    update_watch_time: ""
  vm_resources: null
  vm_type: small
- azs:
  - z1
  - z2
  env:
    bosh:
      agent:
        settings: {}
      ipv6:
        enable: false
  name: log-api
name: scf-dev
variables: []`

// BPMReleaseWithGlobalUpdateBlock contains a manifest with a global update block
const BPMReleaseWithGlobalUpdateBlock = `
name: bpm

releases:
- name: bpm
  version: 1.0.4
  url: docker.io/cfcontainerization
  stemcell:
    os: opensuse-42.3
    version: 36.g03b4653-30.80-7.0.0_316.gcf9fe4a7
update:
  serial: false
  canary_watch_time: 20000-1200000
instance_groups:
- name: bpm1
  jobs:
  - name: test-server
    release: bpm
    properties:
      quarks:
        ports:
        - name: test-server
          protocol: TCP
          internal: 1337
- name: bpm2
  update:
    serial: true
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
- name: bpm3
  update:
    canary_watch_time: 10000-9900000
  jobs:
  - name: test-server3
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
`

// BPMReleaseWithUpdateSerial contains a manifest with some dependent instance groups
const BPMReleaseWithUpdateSerial = `
name: bpm

releases:
- name: bpm
  version: 1.0.4
  url: docker.io/cfcontainerization
  stemcell:
    os: opensuse-42.3
    version: 36.g03b4653-30.80-7.0.0_316.gcf9fe4a7
update:
  serial: false
  canary_watch_time: 20000-1200000
instance_groups:
- name: bpm1
  update:
    serial: true
  jobs:
  - name: test-server
    release: bpm
    properties:
      quarks:
        ports:
        - name: test-server
          protocol: TCP
          internal: 1337
- name: bpm2
  update:
    serial: false
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
- name: bpm3
  update:
    serial: false
  jobs:
  - name: test-server3
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
- name: bpm4
  update:
    serial: true
  jobs:
  - name: test-server4
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

`

// BPMReleaseWithUpdateSerialInManifest contains a manifest with bosh serial on manifest level
const BPMReleaseWithUpdateSerialInManifest = `
name: bpm

releases:
- name: bpm
  version: 1.0.4
  url: docker.io/cfcontainerization
  stemcell:
    os: opensuse-42.3
    version: 36.g03b4653-30.80-7.0.0_316.gcf9fe4a7
update:
  canary_watch_time: 20000-1200000
  serial: false
instance_groups:
- name: bpm1
  jobs:
  - name: test-server
    release: bpm
    properties:
      quarks:
        ports:
        - name: test-server
          protocol: TCP
          internal: 1337
- name: bpm2
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
`

// BPMReleaseWithUpdateSerialAndWithoutPorts contains a manifest with serial but without ports
const BPMReleaseWithUpdateSerialAndWithoutPorts = `
name: bpm

releases:
- name: bpm
  version: 1.0.4
  url: docker.io/cfcontainerization
  stemcell:
    os: opensuse-42.3
    version: 36.g03b4653-30.80-7.0.0_316.gcf9fe4a7
update:
  canary_watch_time: 20000-1200000
  serial: true
instance_groups:
- name: bpm1
  jobs:
  - name: test-server
    release: bpm
    properties:
      quarks:
        ports:
        - name: test-server
          protocol: TCP
          internal: 1337
- name: bpm2
  jobs:
  - name: test-server
    release: bpm
- name: bpm3
  update:
    serial: true
  jobs:
  - name: test-server
    release: bpm
`

// WithNilConsume is a manifest with a nil consume
const WithNilConsume = `---
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
      # 
      log-cache: nil
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
