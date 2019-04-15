## Use Cases

**exstatefulset_configs.yaml**   : Exstatefulset configs will create two extendedstatefulset pods. exstatefulset_configs_updated.yaml is a copy of the **exstatefulset_configs.yaml** except the value of key1 in config1 ConfigMap is changed. When **exstatefulset_configs_updated.yaml** is applied using kubectl, the statefulset pods get updated with the new value of the of key1 in the environment.
**exstatefulset_configs.yaml**   : Exstatefulset azs will create 4 extendedstatefulset pods, 2 in one zone and 2 in another zone as specified in the config yaml.

