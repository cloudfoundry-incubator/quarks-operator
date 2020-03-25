# Process Control

## Background & Motivation

Before `kubecf` the processes of the jobs in an instance group were
managed by `monit`.

This allowed a human operator to suspend (kill) and later restart
these processes as a means of preventing them from interfering with
low-level operations like restoring a cluster using raw database
backups, and the like. Such suspensions were also not visible at kube
level as the pod and container kept running, except through live- and
readiness-probes.

The process control features added to the `containerun` helper
application of the operator serve the same purpose.

## Interface

The process control features of `containerrun` are accessible through
an unix domain __datagram__ socket at location
`/var/vcap/data/containerrun.sock` in the container. Due to this
placement the feature is not accessible from outside a cluster. An
operator (or script written by such) has to log into the relevant
container(s) to use the feature.

  - Suspending the monitored child processes is done by sending the
    command `STOP` to this socket.

  - Conversely, restarting the child processes is done by sending the
    command `START` to this socket.

  - Sending a `START` command when the child processes are running has
    no effect. Conversely the same is true for sending a `STOP`
    command when the child processes are suspended already.

  - Any other command sent to the socket is ignored.

Any tool able to send datagram packet to a unix domain socket of that
type should work.

Examples using `netcat`:

  - `echo START | nc --unixsock --udp /var/vcap/data/containerrun.sock`
  - `echo STOP  | nc --unixsock --udp /var/vcap/data/containerrun.sock`
