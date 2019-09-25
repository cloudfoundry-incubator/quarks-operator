## Use Cases

- [Use Cases](#use-cases)
  - [exjob_output.yaml](#exjoboutputyaml)
  - [exjob_errand.yaml](#exjoberrandyaml)
  - [exjob_auto-errand.yaml](#exjobauto-errandyaml)
  - [exjob_auto-errand-updating.yaml](#exjobauto-errand-updatingyaml)
  - [exjob_auto-errand-deletes-pod.yaml](#exjobauto-errand-deletes-podyaml)

### exjob_output.yaml

This creates a `Secret` from the /mnt/quarks/output.json file in the container volume mount /mnt/quarks.

### exjob_errand.yaml

This exemplifies an errand that needs ot be run manually by the user. This is done by changing the trigger value to `now`.

```shell
kubectl patch ejob \
    -n NAMESPACE manual-sleep \
    -p '{"spec": {"trigger":{"strategy":"now"}}}'
```

### exjob_auto-errand.yaml

This creates a `Job` that runs once, to completion.

### exjob_auto-errand-updating.yaml

This demonstrates the capability to re-run an automated errand when a `ConfigMap` or `Secret` changes.

When `exjob_auto-errand-updating_updated.yaml` is applied, a new `Job` is created.

### exjob_auto-errand-deletes-pod.yaml

This auto-errand will automatically cleanup the completed pod once the `Job` runs successfully.
