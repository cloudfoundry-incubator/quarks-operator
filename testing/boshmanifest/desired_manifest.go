package boshmanifest

// DesiredManifest is the generated desired manifest from the nats deployment
const DesiredManifest = `---
name: nats-deployment
addons_applied: true
director_uuid: ""
instance_groups:
- azs: null
  env:
    bosh:
      agent:
        settings: {}
      ipv6:
        enable: false
  instances: 2
  jobs:
  - name: nats
    properties:
      nats:
        password: custom_password
        user: admin
      quarks:
        consumes: null
        debug: false
        envs: null
        instances: null
        is_addon: false
        ports: null
        post_start: {}
        pre_render_scripts:
          bpm: null
          ig_resolver: null
          jobs: null
        release: ""
        run:
          healthcheck: null
          security_context: null
    release: nats
  name: nats
  properties:
    quarks: {}
  stemcell: ""
  vm_resources: null
releases:
- name: nats
  stemcell:
    os: opensuse-42.3
    version: 30.g9c91e77-30.80-7.0.0_257.gb97ced55
  url: docker.io/cfcontainerization
  version: "26"
`
