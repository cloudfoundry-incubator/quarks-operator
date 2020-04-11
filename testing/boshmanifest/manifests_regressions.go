package boshmanifest

// The manifests in this file are only used in unit tests.  They might not be
// completely valid, miss implicit variables and docker images are may not be
// available.

// They represent specific scenarios for past bugs

// FromKubeCF641 is a manifest generated for KubeCF by the operator, which resulted in ports
// not being generated in BPM secrets, which led to services not being created
// https://github.com/cloudfoundry-incubator/kubecf/pull/641
const FromKubeCF641 = `
addons_applied: true
director_uuid: ""
instance_groups:
- azs: []
  env:
    bosh:
      agent:
        settings: {}
      ipv6:
        enable: false
  instances: 1
  jobs:
  - name: postgres
    properties:
      databases:
        databases:
        - name: autoscaler
          tag: default
        port: 5432
        roles:
        - name: postgres
          password: YUskRrXgEN3exESmKAkK4GEmyt5YXUsg9MBGky5A5zFlX9aWFQxJW19zC5E2ymFG
          tag: default
        tls:
          ca: ca2c629d0ed294dcb4d0a92eb7ecefe2b14f7ede
          certificate: e9555d89370eeeb28b45fbbb3baa89cd61fab4d4
          private_key: 829f9baab55b5dfd2358b1da12c7181fa45c7701
      quarks:
        bpm:
          activePassiveProbes: {}
          debug: false
          post_start: {}
          processes:
          - args:
            - '-'
            - vcap
            - -c
            - |-
              #!/usr/bin/env bash

              set -o errexit -o nounset

              wait_for_file() {
                local file_path="$1"
                local timeout="${2:-30}"
                timeout "${timeout}" bash -c "until [[ -f '${file_path}' ]]; do sleep 1; done"
                return 0
              }

              # shellcheck disable=SC1091
              source /var/vcap/jobs/postgres/bin/pgconfig.sh

              # fixes permissions issue for autoscaler db.
              # https://github.com/cloudfoundry-incubator/kubecf/issues/408
              chmod --recursive 0700 /var/vcap/store/postgres/

              /var/vcap/jobs/postgres/bin/postgres_ctl start
              wait_for_file "${PIDFILE}" || {
                echo "${PIDFILE} did not get created"
                exit 1
              }
              trap '/var/vcap/jobs/postgres/bin/postgres_ctl stop' EXIT

              /var/vcap/jobs/postgres/bin/pg_janitor_ctl start &

              tail \
                --pid "$(cat "${PIDFILE}")" \
                --follow \
                "${LOG_DIR}/startup.log" \
                "${LOG_DIR}/pg_janitor_ctl.log" \
                "${LOG_DIR}/pg_janitor_ctl.err.log"
            executable: /usr/bin/su
            hooks: {}
            limits:
              open_files: 1048576
            name: postgres
            persistent_disk: true
            unsafe: {}
          run:
            healthcheck: {}
            security_context: null
          unsupported_template: false
        consumes: {}
        debug: false
        envs: []
        instances: []
        is_addon: false
        ports:
        - internal: 5432
          name: postgres
          protocol: TCP
        post_start: {}
        pre_render_scripts:
          bpm: []
          ig_resolver: []
          jobs: []
        release: ""
        run:
          healthcheck:
            postgres:
              liveness: null
              readiness:
                exec:
                  command:
                  - /var/vcap/packages/postgres-11.5/bin/pg_isready
          security_context: null
    release: postgres
  name: asdatabase
  networks:
  - name: default
  persistent_disk: 20480
  properties:
    quarks: {}
  stemcell: default
  update:
    canaries: 1
    canary_watch_time: 30000-1200000
    max_in_flight: "1"
    serial: false
    update_watch_time: 5000-1200000
  vm_resources: null
`
