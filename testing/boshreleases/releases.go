package boshreleases

// MultiProcessBPMConfig is a BOSH Job configuration with multi processes
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
  persistent_disk: false
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

// DefaultBPMConfig is a BOSH Job configuration with default values
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
  persistent_disk: false
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

// EnablePersistentDiskBPMConfig is a BOSH Job configuration with persistent disks
const EnablePersistentDiskBPMConfig = `
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

// MultiProcessBPMConfigWithPersistentDisk is a BOSH Job configuration with multi processes and persistent disks
const MultiProcessBPMConfigWithPersistentDisk = `
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
  persistent_disk: true
`

// CFRouting is a BOSH Job configuration that specify cf route job
const CFRouting = `
processes:
  - name: route_registrar
    executable: /var/vcap/packages/route_registrar/bin/route-registrar
    args:
    - --configPath
    - /var/vcap/jobs/route_registrar/config/registrar_settings.json
    - -timeFormat
    - rfc3339
`
