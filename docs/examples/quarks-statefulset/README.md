## Use Cases

- [Use Cases](#use-cases)
  - [qstatefulset_configs.yaml](#qstatefulset_configsyaml)
  - [qstatefulset_configs_updated.yaml](#qstatefulset_configs_updatedyaml)
  - [qstatefulset_azs.yaml](#qstatefulset_azsyaml)
  - [qstatefulset_pvcs.yaml](#qstatefulset_pvcsyaml)
  - [qstatefulset_tolerations.yaml](#qstatefulset_tolerationsyaml)

### qstatefulset_configs.yaml

This creates a `StatefulSet` with two `Pods`.

### qstatefulset_configs_updated.yaml

This is a copy of `qstatefulset_configs.yaml`, with one config value changed. 

When applied on top using `kubectl`, this exemplifies the automatic updating of the `Pods` with a new value for the `SPECIAL_KEY` environment variable.

### qstatefulset_azs.yaml

This creates 4 `Pods` - 2 in one zone and 2 in another zone.

### qstatefulset_pvcs.yaml

This creates `Statefulset Pods` with `Persistent Volumes Claims` attached to each `Pod`. The created `Persistent Volume Claims` get re-attached to the new versions of StatefulSet Pods when the QuarksStatefulSet is updated.

### qstatefulset_tolerations.yaml

This creates `Statefulset Pods` on nodes respecting the tolerations defined on pods and taints defined on nodes.
