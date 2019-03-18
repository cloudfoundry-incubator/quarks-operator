package boshdeployment

import (
	"fmt"
	"reflect"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		StringData: map[string]string{
			"manifest.yaml": string(tempManifestBytes),
		},
	}

	err = r.client.Create(ctx, tempManifestSecret)
	if err != nil {
		r.log.Error("Failed to create temp manifest secret")
		return reconcile.Result{}, err
	}

	// TODO Need to update instanceState after finishing Variable Generation stuff
	instanceState = "Variable Generation"

	// TODO example implementation replace eventually
	varSecrets := []esv1.ExtendedSecret{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "variable-",
				Namespace: request.Namespace,
			},
			Spec: esv1.ExtendedSecretSpec{
				Type: esv1.Password,
			},
		},
	}
	for _, varSecret := range varSecrets {
		err = r.client.Create(ctx, &varSecret)
		if err != nil {
			r.log.Errorf("Failed to create variable secret %s: %v", varSecret.GetName(), err)
			return reconcile.Result{}, err
		}
	}

	varIntJob := newJobForVariableInterpolation(tempManifestSecret, varSecrets, request.Namespace)
	// Set BOSHDeployment instance as the owner and controller
	if err := r.setReference(instance, varIntJob, r.scheme); err != nil {
		r.log.Errorf("Failed to set ownerReference for ExtendedJob '%s': %v", varIntJob.GetName(), err)
		r.recorder.Event(instance, corev1.EventTypeWarning, "NewJobForVariableInterpolation Error", err.Error())
		return reconcile.Result{}, err
	}
	// Check if this job already exists
	foundJob := &batchv1.Job{}
	err = r.client.Get(ctx, types.NamespacedName{Name: varIntJob.Name, Namespace: varIntJob.Namespace}, foundJob)
	if err != nil && apierrors.IsNotFound(err) {
		r.log.Infof("Creating a new Job %s/%s\n", varIntJob.Namespace, varIntJob.Name)
		err = r.client.Create(ctx, varIntJob)
		if err != nil {
			r.log.Errorf("Failed to create ExtendedJob '%s': %v", varIntJob.GetName(), err)
			r.recorder.Event(instance, corev1.EventTypeWarning, "CreateJobForVariableInterpolation Error", err.Error())
			return reconcile.Result{}, err
		}

		// Pod created successfully - don't requeue
		return reconcile.Result{}, nil
	} else if err != nil {
		r.log.Errorf("Failed to get ExtendedJob '%s': %v", varIntJob.GetName(), err)
		r.recorder.Event(instance, corev1.EventTypeWarning, "GetJobForVariableInterpolation Error", err.Error())
		return reconcile.Result{}, err
	}
	// TODO placeholder for variable Interpolation
	instanceState = "Variable Interpolation"

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

// newJobForVariableInterpolation returns a job to interpolate variables
func newJobForVariableInterpolation(manifest *corev1.Secret, variables []esv1.ExtendedSecret, namespace string) *ejv1.ExtendedJob {
	cmd := []string{"cf-operator variable-interpolation -m /var/run/secrets/manifest/manifest.yml -v /var/run/secrets/variables"}
	secretLabels := map[string]string{
		"kind": "temp-manifest",
	}

	volumes := []corev1.Volume{
		{
			Name: manifest.GetName(),
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{

					SecretName: manifest.GetName(),
				},
			},
		},
	}
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      manifest.GetName(),
			MountPath: "/var/run/secrets/manifest",
			ReadOnly:  true,
		},
	}

	for _, variable := range variables {
		volMount := corev1.VolumeMount{
			Name:      variable.GetName(),
			MountPath: "/var/run/secrets/variables/" + variable.GetName(),
			ReadOnly:  true,
		}
		volumeMounts = append(volumeMounts, volMount)

		vol := corev1.Volume{
			Name: variable.GetName(),
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: variable.GetName(),
				},
			},
		}
		volumes = append(volumes, vol)
	}
	one := int64(1)
	job := ejv1.ExtendedJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "variables-interpolation" + "-job",
			Namespace: namespace,
		},
		Spec: ejv1.ExtendedJobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy:                 corev1.RestartPolicyOnFailure,
					TerminationGracePeriodSeconds: &one,
					Containers: []corev1.Container{
						{
							Name:         "variables-interpolation",
							Image:        bdm.GetOperatorDockerImage(),
							Command:      cmd,
							VolumeMounts: volumeMounts,
						},
					},
					Volumes: volumes,
				},
			},
			Output: &ejv1.Output{
				NamePrefix:   "temp-manifest-",
				SecretLabels: secretLabels,
			},
		},
	}
	return &job
}
