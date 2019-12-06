package boshmanifest

// InstanceGroupManifest is the fully rendered instance group manifest,
// captured from the nats deployment
const InstanceGroupManifest = `---
director_uuid: ""
instance_groups:
- azs: null
  env:
    bosh:
      agent:
        settings: {}
      ipv6:
        enable: false
  instances: 0
  jobs:
  - name: nats
    properties:
      nats:
        password: custom_password
        user: admin
      quarks:
        consumes:
          nats:
            address: nats-deployment-nats
            instances:
            - address: nats-deployment-nats-0
              az: ""
              bootstrap: true
              id: nats-0
              index: 0
              instance: 0
              name: nats-nats
              networks: null
            - address: nats-deployment-nats-1
              az: ""
              bootstrap: false
              id: nats-1
              index: 1
              instance: 1
              name: nats-nats
              networks: null
            properties:
              nats:
                password: custom_password
                port: 4222
                user: admin
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
name: nats-deployment
`
