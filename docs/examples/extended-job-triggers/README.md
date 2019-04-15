## Use Cases

**exjob_trigger_ready.yaml**           : Exjob trigger ready will trigger a exjob pod whenever a pod enters into ready state.
**exjob_trigger_deleted.yaml**         : Exjob trigger delete will trigger a exjob pod whenever a pod gets terminated.
**exjob_output.yaml**                  : Exjob output will generate a secret with the data outputed by the exjob pod.
**exjob_errand.yaml**                  : Exjob errand is run manually by the user by changing the trigger value to now as in exjob_errand_updated.yaml. This will run only once.
**exjob_auto-errand.yaml**             : Exjob auto errand will trigger an exjob pod once and ignores it in its completed state.
**exjob_auto-errand-updating.yaml**    : Exjob auto errand updating will create an exjob pod. When **exjob_auto-errand-updating_updating.yaml** is applied using kubectl, a new exjob pod is created. This is because in exjob_auto-errand-updating_updating.yaml, the value of data in config1 is updated which is used by **exjob_auto-errand-updating.yaml**.
**exjob_auto-errand-deletes-pod.yaml** : Exjob auto errand delete will delete the exjob pod after it has run its commmand specified in the config yaml.
**exjob_secret.yaml**                  : Exjob secret makes the operator generate a custom password.
