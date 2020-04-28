# QuarksSecret

- [QuarksSecret](#quarkssecret)
  - [Description](#description)
      - [Watches in Quarks Restart Controller](#watches-in-quarks-restart-controller)
      - [Reconciliation in Quarks Restart Controller](#reconciliation-in-quarks-restart-controller)
  - [`QuarksRestart` Examples](#quarksrestart-examples)

## Description

The QuarksRestart controller is responsible for restarting kubernetes resources such as `StatefulSet` and `Deployment`. They are restarted whenever a secret referenced by these resources gets updated. 

This feature also enables updating entangled pods whenever the link secrets get updated.

#### Watches in Quarks Restart Controller

- `Secret`: Updates which have the annotation `quarks.cloudfoundry.org/update-referenced-owner`

#### Reconciliation in Quarks Restart Controller

- adds restart annotation `quarks.cloudfoundry.org/restart-by-quarks` to `StatefulSet` or `Deployment` as appropriate.


## `QuarksRestart` Examples

See https://github.com/cloudfoundry-incubator/cf-operator/tree/master/docs/examples/quarks-restart
