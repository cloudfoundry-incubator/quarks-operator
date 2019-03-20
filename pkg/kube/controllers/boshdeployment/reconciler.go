package boshdeployment

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"reflect"
	"text/template"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	bdc "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	esv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedsecret/v1alpha1"
	estsv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/context"
)

const (
	passwordSecretTemplate = `---
{{ .VariableName }}: {{ .VariableValue }}
`
	varInterpolationContainerName    = "variables-interpolation"
	varInterpolationOutputNamePrefix = "manifest-"
)

// Check that ReconcileBOSHDeployment implements the reconcile.Reconciler interface
var _ reconcile.Reconciler = &ReconcileBOSHDeployment{}

type setReferenceFunc func(owner, object metav1.Object, scheme *runtime.Scheme) error

// NewReconciler returns a new reconcile.Reconciler
func NewReconciler(log *zap.SugaredLogger, ctrConfig *context.Config, mgr manager.Manager, resolver bdm.Resolver, srf setReferenceFunc) reconcile.Reconciler {

	reconcilerLog := log.Named("boshdeployment-reconciler")
	reconcilerLog.Info("Creating a reconciler for BoshDeployment")

	return &ReconcileBOSHDeployment{
		log:          reconcilerLog,
		ctrConfig:    ctrConfig,
		client:       mgr.GetClient(),
		scheme:       mgr.GetScheme(),
		recorder:     mgr.GetRecorder("RECONCILER RECORDER"),
		resolver:     resolver,
		setReference: srf,
	}
}

// ReconcileBOSHDeployment reconciles a BOSHDeployment object
type ReconcileBOSHDeployment struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client       client.Client
	scheme       *runtime.Scheme
	recorder     record.EventRecorder
	resolver     bdm.Resolver
	setReference setReferenceFunc
	log          *zap.SugaredLogger
	ctrConfig    *context.Config
}

type passwordSecretConfig struct {
	VariableName  string
	VariableValue string
}

// Reconcile reads that state of the cluster for a BOSHDeployment object and makes changes based on the state read
// and what is in the BOSHDeployment.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileBOSHDeployment) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	r.log.Infof("Reconciling BOSHDeployment %s\n", request.NamespacedName)

	// Fetch the BOSHDeployment instance
	instance := &bdc.BOSHDeployment{}

	// Set the ctx to be Background, as the top-level context for incoming requests.
	ctx, cancel := context.NewBackgroundContextWithTimeout(r.ctrConfig.CtxType, r.ctrConfig.CtxTimeOut)
	defer cancel()

	err := r.client.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			r.log.Debug("Skip reconcile: CRD not found\n")
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		r.recorder.Event(instance, corev1.EventTypeWarning, "GetCRD Error", err.Error())
		r.log.Errorf("Failed to get BOSHDeployment '%s': %v", request.NamespacedName, err)
		return reconcile.Result{}, err
	}

	// Get state from instance
	instanceState := instance.Status.State
	if instanceState == "" {
		instanceState = "Created"
	}

	defer func() {
		key := types.NamespacedName{Namespace: instance.GetNamespace(), Name: instance.GetName()}
		err := r.client.Get(ctx, key, instance)
		if err != nil {
			r.log.Errorf("Failed to get BOSHDeployment '%s': %v", instance.GetName(), err)
		}

		// Update the Status of the resource
		if !reflect.DeepEqual(instanceState, instance.Status.State) {
			// Fetch latest BOSHDeployment before update
			instance.Status.State = instanceState
			updateErr := r.client.Update(ctx, instance)
			if updateErr != nil {
				r.log.Errorf("Failed to update BOSHDeployment instance status: %v", updateErr)
			}
		}
	}()

	// retrieve manifest
	instanceState = "Applying Ops Files"
	manifest, err := r.resolver.ResolveManifest(instance.Spec, request.Namespace)
	if err != nil {
		r.recorder.Event(instance, corev1.EventTypeWarning, "ResolveCRD Error", err.Error())
		r.log.Errorf("Error resolving the manifest %s: %s", request.NamespacedName, err)
		return reconcile.Result{}, err
	}

	if len(manifest.InstanceGroups) < 1 {
		err = fmt.Errorf("manifest is missing instance groups")
		r.log.Errorf("No instance groups defined in manifest %s", request.NamespacedName)
		r.recorder.Event(instance, corev1.EventTypeWarning, "MissingInstance Error", err.Error())
		return reconcile.Result{}, err
	}

	kubeConfigs, err := manifest.ConvertToKube(r.ctrConfig.Namespace)
	if err != nil {
		r.log.Errorf("Error converting bosh manifest %s to kube objects: %s", request.NamespacedName, err)
		r.recorder.Event(instance, corev1.EventTypeWarning, "BadManifest Error", err.Error())
		return reconcile.Result{}, errors.Wrap(err, "error converting manifest to kube objects")
	}

	tempManifestBytes, err := yaml.Marshal(manifest)
	if err != nil {
		r.log.Error("Failed to marshal temp manifest")
		return reconcile.Result{}, err
	}

	tempManifestSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      manifest.GenerateSecretName(manifest.Name),
			Namespace: request.Namespace,
		},
		StringData: map[string]string{
			"manifest.yaml": string(tempManifestBytes),
		},
	}

	// Check if temp manifest secret already exists
	foundSecret := &corev1.Secret{}
	err = r.client.Get(ctx, types.NamespacedName{Name: tempManifestSecret.Name, Namespace: tempManifestSecret.Namespace}, foundSecret)
	if err != nil && apierrors.IsNotFound(err) {
		r.log.Infof("Creating a new temp manifest secret %s/%s\n", tempManifestSecret.Namespace, tempManifestSecret.Name)
		if err := r.setReference(instance, tempManifestSecret, r.scheme); err != nil {
			return reconcile.Result{}, err
		}
		err = r.client.Create(ctx, tempManifestSecret)
		if err != nil {
			r.log.Errorf("Failed to create temp manifest secret: %v", err)
			return reconcile.Result{}, err
		}

		return reconcile.Result{Requeue: true}, err
	} else if err != nil {
		r.log.Errorf("Failed to get secret '%s': %v", tempManifestSecret.GetName(), err)
		r.recorder.Event(instance, corev1.EventTypeWarning, "GetJobForVariableInterpolation Error", err.Error())
		return reconcile.Result{}, err
	} else {
		foundSecret.StringData = tempManifestSecret.StringData
		err = r.client.Update(ctx, foundSecret)
		if err != nil {
			r.log.Errorf("Failed to update temp manifest secret: %v", err)
			return reconcile.Result{}, err
		}
	}

	// TODO Need to update instanceState after finishing Variable Generation stuff
	instanceState = "Variable Generation"

	// TODO example implementation replace eventually
	varSecrets := []esv1.ExtendedSecret{}
	for _, variable := range manifest.Variables {
		varSecret := esv1.ExtendedSecret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      manifest.GenerateSecretName(variable.Name),
				Namespace: request.Namespace,
				Labels: map[string]string{
					"variableName": variable.Name,
				},
			},
			Spec: esv1.ExtendedSecretSpec{
				SecretName: manifest.GenerateSecretName(variable.Name),
				Type:       esv1.Type(variable.Type),
			},
		}

		// Check if variable extendedSecret already exists
		foundExSecret := &esv1.ExtendedSecret{}
		err = r.client.Get(ctx, types.NamespacedName{Name: varSecret.Name, Namespace: varSecret.Namespace}, foundExSecret)
		if err != nil && apierrors.IsNotFound(err) {
			r.log.Infof("Creating a new temp manifest extendedSecret %s/%s\n", varSecret.Namespace, varSecret.Name)
			if err := r.setReference(instance, &varSecret, r.scheme); err != nil {
				return reconcile.Result{}, err
			}
			err = r.client.Create(ctx, &varSecret)
			if err != nil {
				r.log.Errorf("Failed to create temp manifest extendedSecret: %v", err)
				return reconcile.Result{}, err
			}

			return reconcile.Result{Requeue: true}, err
		} else if err != nil {
			r.log.Errorf("Failed to get extendedSecret '%s': %v", varSecret.GetName(), err)
			r.recorder.Event(instance, corev1.EventTypeWarning, "GetJobForVariableInterpolation Error", err.Error())
			return reconcile.Result{}, err
		} else {
			foundExSecret.Spec = varSecret.Spec
			err = r.client.Update(ctx, foundExSecret)
			if err != nil {
				r.log.Errorf("Failed to Update extendedSecret '%s': %v", varSecret.GetName(), err)
				return reconcile.Result{}, err
			}
		}
		varSecrets = append(varSecrets, *foundExSecret)
	}

	instanceState = "Variable Interpolation"
	varIntExJob, err := r.newExtendedJobForVariableInterpolation(ctx, foundSecret, varSecrets, request.Namespace)
	if err != nil && apierrors.IsNotFound(err) {
		r.log.Debug("Could not find variable secrets, waiting")
		return reconcile.Result{Requeue: true}, nil
	} else if err != nil {
		r.log.Errorf("Failed to generate variable interpolation job: %s", err)
		return reconcile.Result{}, err
	}
	// Set BOSHDeployment instance as the owner and controller
	if err := r.setReference(instance, varIntExJob, r.scheme); err != nil {
		r.log.Errorf("Failed to set ownerReference for ExtendedJob '%s': %v", varIntExJob.GetName(), err)
		r.recorder.Event(instance, corev1.EventTypeWarning, "NewJobForVariableInterpolation Error", err.Error())
		return reconcile.Result{}, err
	}

	// Check if this job already exists
	foundExJob := &ejv1.ExtendedJob{}
	err = r.client.Get(ctx, types.NamespacedName{Name: varIntExJob.Name, Namespace: varIntExJob.Namespace}, foundExJob)
	if err != nil && apierrors.IsNotFound(err) {
		r.log.Infof("Creating a new Job %s/%s\n", varIntExJob.Namespace, varIntExJob.Name)
		err = r.client.Create(ctx, varIntExJob)
		if err != nil {
			r.log.Errorf("Failed to create ExtendedJob '%s': %v", varIntExJob.GetName(), err)
			r.recorder.Event(instance, corev1.EventTypeWarning, "CreateJobForVariableInterpolation Error", err.Error())
			return reconcile.Result{}, err
		}

		return reconcile.Result{Requeue: true}, nil
	} else if err != nil {
		r.log.Errorf("Failed to get ExtendedJob '%s': %v", varIntExJob.GetName(), err)
		r.recorder.Event(instance, corev1.EventTypeWarning, "GetJobForVariableInterpolation Error", err.Error())
		return reconcile.Result{}, err
	}

	// Check job deletion and secret creation
	jobs, err := r.listJobs(ctx, foundExJob)
	if err != nil {
		r.log.Errorf("Failed to get jobs owned by '%s': %v", foundExJob.GetName(), err)
		return reconcile.Result{Requeue: true}, err
	}

	if len(jobs) != 0 {
		r.log.Debugf("Waiting for ExtendedJob '%s' to finish", foundExJob.GetName())
		return reconcile.Result{Requeue: true, RequeueAfter: 1 * time.Second}, nil
	}

	varIntExJobSecret := &corev1.Secret{}
	err = r.client.Get(ctx, types.NamespacedName{Name: varInterpolationOutputNamePrefix + varInterpolationContainerName, Namespace: instance.Namespace}, varIntExJobSecret)
	if err != nil && apierrors.IsNotFound(err) {
		r.log.Debugf("Waiting for desired manifest secret '%s' to create", varInterpolationOutputNamePrefix+varInterpolationContainerName)
		return reconcile.Result{Requeue: true, RequeueAfter: 1 * time.Second}, nil
	} else if err != nil {
		r.log.Errorf("Failed to get secret '%s': %v", varInterpolationOutputNamePrefix+varInterpolationContainerName, err)
		r.recorder.Event(instance, corev1.EventTypeWarning, "GetJobForVariableInterpolation Error", err.Error())
		return reconcile.Result{}, err
	}

	encodedDesiredManifest, exists := varIntExJobSecret.Data["interpolated-manifest.yaml"]
	if !exists {
		r.log.Errorf("Failed to get desiredManifest value from secret")
		return reconcile.Result{}, err
	}
	desiredManifestBytes, err := base64.StdEncoding.DecodeString(string(encodedDesiredManifest))
	if err != nil {
		r.log.Errorf("Failed to decode desiredManifest string: %v", err)
		return reconcile.Result{}, err
	}

	desiredManifest := &bdm.Manifest{}
	err = yaml.Unmarshal(desiredManifestBytes, desiredManifest)
	if err != nil {
		r.log.Error("Failed to unmarshal desired manifest")
		return reconcile.Result{}, err
	}

	// TODO Need to update instanceState after finishing Data Gathering stuff

	for _, eJob := range kubeConfigs.ExtendedJob {
		// Set BOSHDeployment instance as the owner and controller
		if err := r.setReference(instance, &eJob, r.scheme); err != nil {
			r.recorder.Event(instance, corev1.EventTypeWarning, "NewExtendedJobForDeployment Error", err.Error())
			return reconcile.Result{}, errors.Wrap(err, "couldn't set reference for an ExtendedJob for a BOSH Deployment")
		}

		// Check to see if the object already exists
		existingEJob := &ejv1.ExtendedJob{}
		err = r.client.Get(ctx, types.NamespacedName{Name: eJob.Name, Namespace: eJob.Namespace}, existingEJob)
		if err != nil && apierrors.IsNotFound(err) {
			r.log.Infof("Creating a new ExtendedJob %s/%s for Deployment Manifest %s\n", eJob.Namespace, eJob.Name, instance.Name)

			// Create the extended job
			err := r.client.Create(ctx, &eJob)
			if err != nil {
				r.recorder.Event(instance, corev1.EventTypeWarning, "CreateExtendedJobForDeployment Error", err.Error())
				r.log.Errorf("Error creating ExtendedJob %s for deployment manifest %s: %s", eJob.Name, request.NamespacedName, err)
				return reconcile.Result{}, errors.Wrap(err, "couldn't create an ExtendedJob for a BOSH Deployment")
			}
		}
	}

	for _, eSts := range kubeConfigs.ExtendedSts {
		// Set BOSHDeployment instance as the owner and controller
		if err := r.setReference(instance, &eSts, r.scheme); err != nil {
			r.recorder.Event(instance, corev1.EventTypeWarning, "NewExtendedStatefulSetForDeployment Error", err.Error())
			return reconcile.Result{}, errors.Wrap(err, "couldn't set reference for an ExtendedStatefulSet for a BOSH Deployment")
		}

		// Check to see if the object already exists
		existingESts := &estsv1.ExtendedStatefulSet{}
		err = r.client.Get(ctx, types.NamespacedName{Name: eSts.Name, Namespace: eSts.Namespace}, existingESts)
		if err != nil && apierrors.IsNotFound(err) {
			r.log.Infof("Creating a new ExtendedStatefulset %s/%s for Deployment Manifest %s\n", eSts.Namespace, eSts.Name, instance.Name)

			// Create the extended statefulset
			err := r.client.Create(ctx, &eSts)
			if err != nil {
				r.recorder.Event(instance, corev1.EventTypeWarning, "CreateExtendedStatefulSetForDeployment Error", err.Error())
				r.log.Errorf("Error creating ExtendedStatefulSet %s for deployment manifest %s: %s", eSts.Name, request.NamespacedName, err)
				return reconcile.Result{}, errors.Wrap(err, "couldn't create an ExtendedStatefulSet for a BOSH Deployment")
			}
		}
	}

	return reconcile.Result{}, nil
}

// newExtendedJobForVariableInterpolation returns a job to interpolate variables
func (r *ReconcileBOSHDeployment) newExtendedJobForVariableInterpolation(ctx context.Context, manifest *corev1.Secret, variables []esv1.ExtendedSecret, namespace string) (*ejv1.ExtendedJob, error) {
	cmd := []string{"/bin/sh"}
	args := []string{"-c", "cf-operator variable-interpolation --manifest /var/run/secrets/manifest/manifest.yaml --variables-dir /var/run/secrets/variables | base64 | tr -d '\n' | echo \"{\\\"interpolated-manifest.yaml\\\":\\\"$(</dev/stdin)\\\"}\""}
	secretLabels := map[string]string{
		"kind": "manifest",
	}

	volumes := []corev1.Volume{
		{
			Name: string(manifest.GetUID()),
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: manifest.GetName(),
					Items: []corev1.KeyToPath{
						{
							Key:  "manifest.yaml",
							Path: "manifest.yaml",
						},
					},
				},
			},
		},
	}
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      string(manifest.GetUID()),
			MountPath: "/var/run/secrets/manifest",
			ReadOnly:  true,
		},
	}

	for _, variable := range variables {
		secretItems := []corev1.KeyToPath{}
		// Update secret and
		switch variable.Spec.Type {
		case esv1.Password:
			varName := variable.GetLabels()["variableName"]

			// Check if temp manifest secret already exists
			foundSecret := &corev1.Secret{}
			err := r.client.Get(ctx, types.NamespacedName{Name: variable.Name, Namespace: variable.Namespace}, foundSecret)
			if err != nil && apierrors.IsNotFound(err) {
				r.log.Errorf("Could not find secret '%s'")
				return nil, err
			} else if err != nil {
				r.log.Errorf("Failed to get secret '%s': %v", variable.GetName(), err)
				return nil, err
			}
			varValue := string(foundSecret.Data[string(esv1.Password)])

			passwordSecretValues := passwordSecretConfig{
				VariableName:  varName,
				VariableValue: varValue,
			}

			buffer := bytes.NewBuffer([]byte{})

			t := template.Must(template.New("network-interfaces").Parse(passwordSecretTemplate))

			err = t.Execute(buffer, passwordSecretValues)
			if err != nil {
				return nil, err
			}

			foundSecret.Data = map[string][]byte{}
			foundSecret.StringData = map[string]string{
				string(esv1.Password): buffer.String(),
			}
			err = r.client.Update(ctx, foundSecret)
			if err != nil {
				r.log.Errorf("Failed to Update secret '%s': %v", foundSecret.GetName(), err)
				return nil, err
			}

			secretItems = []corev1.KeyToPath{
				{
					Key:  string(esv1.Password),
					Path: "variable.yaml",
				},
			}
		case esv1.RSAKey:
		case esv1.SSHKey:
		case esv1.Certificate:
		default:
			secretItems = []corev1.KeyToPath{}
		}

		vol := corev1.Volume{
			Name: string(variable.GetUID()),
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: variable.GetName(),
					Items:      secretItems,
				},
			},
		}
		volumes = append(volumes, vol)

		volMount := corev1.VolumeMount{
			Name:      string(variable.GetUID()),
			MountPath: "/var/run/secrets/variables/" + variable.GetName(),
			ReadOnly:  true,
		}
		volumeMounts = append(volumeMounts, volMount)
	}
	one := int64(1)
	job := ejv1.ExtendedJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "variables-interpolation-job",
			Namespace: namespace,
		},
		Spec: ejv1.ExtendedJobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy:                 corev1.RestartPolicyOnFailure,
					TerminationGracePeriodSeconds: &one,
					Containers: []corev1.Container{
						{
							Name:         varInterpolationContainerName,
							Image:        bdm.GetOperatorDockerImage(),
							Command:      cmd,
							Args:         args,
							VolumeMounts: volumeMounts,
						},
					},
					Volumes: volumes,
				},
			},
			Output: &ejv1.Output{
				NamePrefix:   varInterpolationOutputNamePrefix,
				SecretLabels: secretLabels,
			},
			Trigger: ejv1.Trigger{
				Strategy: ejv1.TriggerOnce,
			},
		},
	}
	return &job, nil
}

// listJobs gets all jobs owned by the ExtendedJob
func (r *ReconcileBOSHDeployment) listJobs(ctx context.Context, exJob *ejv1.ExtendedJob) ([]batchv1.Job, error) {
	r.log.Debug("Listing StatefulSets owned by ExtendedStatefulSet '", exJob.Name, "'.")

	result := []batchv1.Job{}

	// Get owned resources
	// Go through each StatefulSet
	allJobs := &batchv1.JobList{}
	err := r.client.List(
		ctx,
		&client.ListOptions{
			Namespace:     exJob.Namespace,
			LabelSelector: labels.Everything(),
		},
		allJobs)
	if err != nil {
		return nil, err
	}

	for _, job := range allJobs.Items {
		if metav1.IsControlledBy(&job, exJob) {
			result = append(result, job)
			r.log.Debugf("Job '%s' is owned by ExtendedJob '%s'", job.Name, exJob.Name)
		} else {
			r.log.Debugf("Job '%s' is not owned by ExtendedJob '%s', ignoring", job.Name, exJob.Name)
		}
	}

	return result, nil
}
