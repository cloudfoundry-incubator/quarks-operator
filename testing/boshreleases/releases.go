package boshreleases

const MultiProcessBPMConfig = `
processes:
- name: test-server
  executable: /var/vcap/packages/test-server/bin/test-server
  args:
    - --port
    - 1337
  env:
    BPM: SWEET
  limits:
    memory: 1G
    open_files: 1024
  ephemeral_disk: true
  persistent_disk: true
  unsafe:
    unrestricted_volumes:
    - path: /dev/log
      mount_only: true

- name: alt-test-server
  executable: /var/vcap/packages/test-server/bin/test-server
  args:
    - --port
    - 1338
    - --ignore-signals
  env:
    BPM: CONTAINED
`

const DefaultBPMConfig = `
processes:
- name: test-server
  executable: /var/vcap/packages/test-server/bin/test-server
  args:
    - --port
    - 1337
  env:
    BPM: SWEET
  limits:
    memory: 1G
    open_files: 1024
  ephemeral_disk: true
  persistent_disk: true
  additional_volumes:
  - path: /var/vcap/data/shared
    writable: true
  - path: /var/vcap/store/foo
    writable: true
  unsafe:
    unrestricted_volumes:
    - path: /dev/log
      mount_only: true
`
