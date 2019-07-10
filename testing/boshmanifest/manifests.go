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
      bosh_containerization:
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

// NatsSmall is a small manifest to start nats
const NatsSmall = `---
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

// NatsSmallWithPatch is a manifest that patches the prestart hook to loop forever
const NatsSmallWithPatch = `---
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
  instances: 1
  jobs:
  - name: nats
    release: nats
    properties:
      nats:
        user: admin
        password: changeme
        debug: true
      bosh_containerization:
        pre_render_scripts:
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
releases:
- name: cf-operator-testing
  version: "0.0.2"
  url: docker.io/cfcontainerization
  stemcell:
    os: opensuse-42.3
    version: 36.g03b4653-30.80-7.0.0_332.g0d8469bb
instance_groups:
- name: drains
  instances: 1
  jobs:
  - name: failing-drain-job
    release: cf-operator-testing
  - name: delaying-drain-job
    release: cf-operator-testing
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
      bosh_containerization:
        ports:
        - name: "redis"
          protocol: "TCP"
          internal: 6379
        pre_render_scripts:
        - |
          touch /tmp
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
      bosh_containerization:
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

// BPMRelease utilizing the test server to open two tcp ports
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

instance_groups:
- name: route_registrar
  instances: 2
  jobs:
  - name: route_registrar
    release: routing
    properties:
      bosh_containerization:
        bpm:
          processes:
          - name: route_registrar
            executable: sleep
            args: ["10"]
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

// GardenRunc BOSH release is being tested for BPM pre start hook
const GardenRunc = `
  name: garden-runc
  releases:
  - name: garden-runc
    version: 1.19.2
    url: docker.io/cfcontainerization
    stemcell:
      os: opensuse-42.3
      version: 36.g03b4653-30.80-7.0.0_332.g0d8469bb
  instance_groups:
  - name: garden-runc
    instances: 2
    jobs:
    - name: garden
      release: garden-runc
      properties:
        containerd_mode: true
        cleanup_process_dirs_on_wait: true
        debug_listen_address: 127.0.0.1:17019
        default_container_grace_time: 0
        destroy_containers_on_start: true
        deny_networks:
        - 0.0.0.0/0
        network_plugin: /var/vcap/packages/runc-cni/bin/garden-external-networker
        network_plugin_extra_args:
        - --configFile=/var/vcap/jobs/garden-cni/config/adapter.json
      logging:
        format:
          timestamp: "rfc3339"
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

const WithMultiBPMProcessesAndPersistentDisk = `---
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
name: bpm

releases:
- name: bpm
  version: 1.0.4
  url: docker.io/cfcontainerization
  stemcell:
    os: opensuse-42.3
    version: 36.g03b4653-30.80-7.0.0_316.gcf9fe4a7

instance_groups:
- name: bpm1
  instances: 2
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
- name: bpm2
  instances: 2
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
- name: bpm3
  instances: 2
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
`
