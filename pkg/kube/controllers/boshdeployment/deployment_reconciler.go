package boshdeployment

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	bpm "code.cloudfoundry.org/cf-operator/pkg/bosh/bpm"
	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/cf-operator/pkg/kube/apis"
	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	esv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedsecret/v1alpha1"
	estsv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	log "code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/finalizer"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/names"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/owner"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/versionedsecretstore"
)

// State of instance
const (
	CreatedState              = "Created"
	UpdatedState              = "Updated"
	OpsAppliedState           = "OpsApplied"
	VariableGeneratedState    = "VariableGenerated"
	VariableInterpolatedState = "VariableInterpolated"
	DataGatheredState         = "DataGathered"
	BPMConfigsCreatedState    = "BPMConfigsCreatedState"
	DeployingState            = "Deploying"
	DeployedState             = "Deployed"
)

// Check that ReconcileBOSHDeployment implements the reconcile.Reconciler interface
var _ reconcile.Reconciler = &ReconcileBOSHDeployment{}

type setReferenceFunc func(owner, object metav1.Object, scheme *runtime.Scheme) error

// Owner bundles funcs to manage ownership on referenced configmaps and secrets
type Owner interface {
	RemoveOwnerReferences(context.Context, apis.Object, []apis.Object) error
	ListConfigsOwnedBy(context.Context, apis.Object) ([]apis.Object, error)
}

// NewReconciler returns a new reconcile.Reconciler
func NewReconciler(ctx context.Context, config *config.Config, mgr manager.Manager, resolver bdm.Resolver, srf setReferenceFunc) reconcile.Reconciler {
	versionedSecretStore := versionedsecretstore.NewVersionedSecretStore(mgr.GetClient())

	return &ReconcileBOSHDeployment{
		ctx:                  ctx,
		config:               config,
		client:               mgr.GetClient(),
		scheme:               mgr.GetScheme(),
		resolver:             resolver,
		setReference:         srf,
		owner:                owner.NewOwner(mgr.GetClient(), mgr.GetScheme()),
		versionedSecretStore: versionedSecretStore,
	}
}

// ReconcileBOSHDeployment reconciles a BOSHDeployment object
type ReconcileBOSHDeployment struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	ctx                  context.Context
	client               client.Client
	scheme               *runtime.Scheme
	resolver             bdm.Resolver
	setReference         setReferenceFunc
	config               *config.Config
	owner                Owner
	versionedSecretStore versionedsecretstore.VersionedSecretStore
}

// Reconcile reads that state of the cluster for a BOSHDeployment object and makes changes based on the state read
// and what is in the BOSHDeployment.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileBOSHDeployment) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the BOSHDeployment instance
	instance := &bdv1.BOSHDeployment{}

	// Set the ctx to be Background, as the top-level context for incoming requests.
	ctx, cancel := context.WithTimeout(r.ctx, r.config.CtxTimeOut)
	defer cancel()

	log.Infof(ctx, "Reconciling BOSHDeployment %s", request.NamespacedName)
	err := r.client.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Debug(ctx, "Skip reconcile: BOSHDeployment not found")
			return reconcile.Result{}, nil
		}
		log.WithEvent(instance, "GetBOSHDeploymentError").Errorf(ctx, "Failed to get BOSHDeployment '%s': %v", request.NamespacedName, err)
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Clean up instance if has been marked for deletion
	if instance.ToBeDeleted() {
		return r.handleDeletion(ctx, instance)
	}

	// Get state from instance
	instanceState := instance.Status.State

	// Resolve the manifest (incl. ops files and implicit variables)
	manifest, err := r.resolveManifest(ctx, instance)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Set manifest and ops ownerReference as instance
	err = r.setSpecsOwnerReference(ctx, instance)
	if err != nil {
		log.Errorf(ctx, "Could not set specs ownerReference for instance '%s': %v", request.NamespacedName, err)
		return reconcile.Result{}, errors.Wrap(err, "could not set specs ownerReference")
	}

	// Add or update the instance's finalizer
	err = r.setFinalizer(ctx, instance)
	if err != nil {
		log.Errorf(ctx, "Could not set finalizer for instance '%s': %v", request.NamespacedName, err)
		return reconcile.Result{}, errors.Wrap(err, "could not set instance's finalizer")
	}

	// Compute SHA1 of the manifest (with ops applied), so we can figure out if anything
	// has changed.
	currentManifestSHA1, err := manifest.SHA1()
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "could not calculate manifest SHA1")
	}
	oldManifestSHA1, _ := instance.Annotations[bdv1.AnnotationManifestSHA1]
	if oldManifestSHA1 == currentManifestSHA1 && instance.Status.State == DeployedState {
		log.WithEvent(instance, "SkipReconcile").Infof(ctx, "Skip reconcile: deployed BoshDeployment '%s/%s' manifest has not changed", instance.GetNamespace(), instance.GetName())
		return reconcile.Result{}, nil
	}

	// If we have no instance groups, we should stop. There must be something wrong
	// with the manifest.
	if len(manifest.InstanceGroups) < 1 {
		err := log.WithEvent(instance, "MissingInstanceError").Errorf(ctx, "No instance groups defined in manifest %s", manifest.Name)
		return reconcile.Result{}, err
	}

	// Generate all the kube objects we need for the manifest
	log.Debug(ctx, "Converting bosh manifest to kube objects")
	kubeConfigs := bdm.NewKubeConfig(r.config.Namespace, manifest.Name, manifest)
	jobFactory := bdm.NewJobFactory(*manifest, instance.GetNamespace())

	if instanceState == "" {
		// Set a "Created" state if this has just been created
		instanceState = CreatedState
	} else if currentManifestSHA1 != oldManifestSHA1 {
		// Set an "Updated" state if the signature of the manifest has changed
		instanceState = UpdatedState
	}

	log.Debugf(ctx, "BoshDeployment '%s/%s' is in state: %s", instance.GetNamespace(), instance.GetName(), instanceState)

	switch instanceState {
	case CreatedState:
		fallthrough
	case UpdatedState:
		// Set manifest SHA1
		if instance.Annotations == nil {
			instance.Annotations = map[string]string{}
		}
		instance.Annotations[bdv1.AnnotationManifestSHA1] = currentManifestSHA1
		instance.Status.State = OpsAppliedState

	case OpsAppliedState:
		secrets := kubeConfigs.Variables(manifest.Variables)
		err = r.generateVariableSecrets(ctx, instance, manifest, secrets)
		if err != nil {
			log.WithEvent(instance, "VariableGenerationError").Errorf(ctx, "Failed to generate variables: %v", err)
			return reconcile.Result{}, err
		}

	case VariableGeneratedState:
		job, err := jobFactory.VariableInterpolationJob()
		if err != nil {
			log.WithEvent(instance, "VariableGenerationError").Errorf(ctx, "Failed to build variable interpolation eJob: %v", err)
			return reconcile.Result{}, err
		}
		err = r.createVariableInterpolationEJob(ctx, instance, manifest, job)
		if err != nil {
			log.WithEvent(instance, "VariableInterpolationError").Errorf(ctx, "Failed to create variable interpolation eJob: %v", err)
			return reconcile.Result{}, err
		}

	case VariableInterpolatedState:
		job, err := jobFactory.DataGatheringJob()
		if err != nil {
			log.WithEvent(instance, "DataGatheringError").Errorf(ctx, "Failed to build data gathering eJob: %v", err)
			return reconcile.Result{}, err
		}
		err = r.createDataGatheringJob(ctx, instance, job)
		if err != nil {
			log.WithEvent(instance, "DataGatheringError").Errorf(ctx, "Failed to create data gathering eJob: %v", err)
			return reconcile.Result{}, err
		}

	case DataGatheredState:
		job, err := jobFactory.BPMConfigsJob()
		if err != nil {
			log.WithEvent(instance, "DataGatheringError").Errorf(ctx, "Failed to build BPM configs eJob: %v", err)
			return reconcile.Result{}, err
		}
		err = r.createBPMConfigsJob(ctx, instance, job)
		if err != nil {
			log.WithEvent(instance, "BPMConfigsError").Errorf(ctx, "Failed to create BPM configs eJob: %v", err)
			return reconcile.Result{}, err
		}

	case BPMConfigsCreatedState:
		// Wait for all instance group property outputs to be ready
		// We need BPM information to start everything up
		job, err := jobFactory.DataGatheringJob()
		if err != nil {
			log.WithEvent(instance, "DataGatheringError").Errorf(ctx, "Failed to build data gathering eJob: %v", err)
			return reconcile.Result{}, err
		}

		bpmInfo, err := r.waitForBPM(ctx, instance, manifest, job.Spec.Template.Spec.Containers)
		if err != nil {
			log.Infof(ctx, "Waiting for BPM: %s", err.Error())
			return reconcile.Result{Requeue: true, RequeueAfter: 5 * time.Second}, err
		}

		err = kubeConfigs.ApplyBPMInfo(manifest.InstanceGroups, bpmInfo)
		if err != nil {
			log.Errorf(ctx, "Failed to apply BPM information: %v", err)
			return reconcile.Result{}, err
		}

		err = r.deployInstanceGroups(ctx, instance, kubeConfigs)
		if err != nil {
			log.Errorf(ctx, "Failed to deploy instance groups: %v", err)
			return reconcile.Result{}, err
		}

	case DeployingState:
		err = r.actionOnDeploying(ctx, instance)
		if err != nil {
			log.WithEvent(instance, "InstanceDeploymentError").Errorf(ctx, "Failed to deploy: %v", err)
			return reconcile.Result{}, err
		}

	case DeployedState:
		log.WithEvent(instance, "SkipReconcile").Infof(ctx, "Skip reconcile: BoshDeployment '%s/%s' already has been deployed", instance.GetNamespace(), instance.GetName())
		return reconcile.Result{}, nil
	default:
		return reconcile.Result{}, errors.New("unknown instance state")
	}

	log.Debugf(ctx, "Requeue the reconcile: BoshDeployment '%s/%s' is in state '%s'", instance.GetNamespace(), instance.GetName(), instance.Status.State)
	return reconcile.Result{Requeue: true}, r.updateInstanceState(ctx, instance)
}

// updateInstanceState update instance state
func (r *ReconcileBOSHDeployment) updateInstanceState(ctx context.Context, currentInstance *bdv1.BOSHDeployment) error {
	currentManifestSHA1, _ := currentInstance.GetAnnotations()[bdv1.AnnotationManifestSHA1]

	// Fetch latest BOSHDeployment before update
	foundInstance := &bdv1.BOSHDeployment{}
	key := types.NamespacedName{Namespace: currentInstance.GetNamespace(), Name: currentInstance.GetName()}
	err := r.client.Get(ctx, key, foundInstance)
	if err != nil {
		log.Errorf(ctx, "Failed to get BOSHDeployment instance '%s': %v", currentInstance.GetName(), err)
		return err
	}
	oldManifestSHA1, _ := foundInstance.GetAnnotations()[bdv1.AnnotationManifestSHA1]

	if oldManifestSHA1 != currentManifestSHA1 {
		// Set manifest SHA1
		if foundInstance.Annotations == nil {
			foundInstance.Annotations = map[string]string{}
		}

		foundInstance.Annotations[bdv1.AnnotationManifestSHA1] = currentManifestSHA1
	}

	// Update the Status of the resource
	if !reflect.DeepEqual(foundInstance.Status.State, currentInstance.Status.State) {
		log.Debugf(ctx, "Updating boshDeployment from '%s' to '%s'", foundInstance.Status.State, currentInstance.Status.State)

		newInstance := foundInstance.DeepCopy()
		newInstance.Status.State = currentInstance.Status.State

		err = r.client.Update(ctx, newInstance)
		if err != nil {
			log.Errorf(ctx, "Failed to update BOSHDeployment instance status: %v", err)
			return err
		}
	}

	return nil
}

// resolveManifest resolves the manifest and applies ops files and implicit variable interpolation
func (r *ReconcileBOSHDeployment) resolveManifest(ctx context.Context, instance *bdv1.BOSHDeployment) (*bdm.Manifest, error) {
	// Create temp manifest as variable interpolation job input
	// retrieve manifest
	log.Debug(ctx, "Resolving manifest")
	manifest, err := r.resolver.ResolveManifest(instance, instance.GetNamespace())
	if err != nil {
		log.WithEvent(instance, "ResolveManifestError").Errorf(ctx, "Error resolving the manifest %s: %s", instance.GetName(), err)
		return nil, err
	}

	// Replace the name with the name of the BOSHDeployment resource
	manifest.Name = instance.GetName()

	return manifest, nil
}

// setSpecsOwnerReference set manifest/ops ownerReference as BOSHDeployment instance
func (r *ReconcileBOSHDeployment) setSpecsOwnerReference(ctx context.Context, instance *bdv1.BOSHDeployment) error {
	var err error

	// Set manifest's ownerReference as BOSHDeployment instance
	refType := instance.Spec.Manifest.Type
	refName := instance.Spec.Manifest.Ref

	switch refType {
	case bdv1.ConfigMapType:
		opsConfig := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      refName,
				Namespace: instance.GetNamespace(),
			},
		}
		err = r.updateObjectWithOwnerReference(ctx, instance, opsConfig)
		if err != nil {
			return errors.Wrapf(err, "failed to update configMap '%s/%s'", refName, instance.GetNamespace())
		}
	case bdv1.SecretType:
		opsSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      refName,
				Namespace: instance.GetNamespace(),
			}}
		err = r.updateObjectWithOwnerReference(ctx, instance, opsSecret)
		if err != nil {
			return errors.Wrapf(err, "failed to update secret '%s/%s'", refName, instance.GetNamespace())
		}
	default:
		log.Debugf(ctx, "unrecognized ref type %s to set ownerReference", refType)
	}

	// Set ops' ownerReference as BOSHDeployment instance
	for _, op := range instance.Spec.Ops {
		refType = op.Type
		refName = op.Ref

		switch refType {
		case bdv1.ConfigMapType:
			opsConfig := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      refName,
					Namespace: instance.GetNamespace(),
				},
			}
			err = r.updateObjectWithOwnerReference(ctx, instance, opsConfig)
			if err != nil {
				return errors.Wrapf(err, "failed to update configMap '%s/%s'", refName, instance.GetNamespace())
			}
		case bdv1.SecretType:
			opsSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      refName,
					Namespace: instance.GetNamespace(),
				}}
			err = r.updateObjectWithOwnerReference(ctx, instance, opsSecret)
			if err != nil {
				return errors.Wrapf(err, "failed to update secret '%s/%s'", refName, instance.GetNamespace())
			}
		default:
			log.Debugf(ctx, "unrecognized ref type %s to set ownerReference", refType)
		}
	}

	return err
}

// updateObjectWithOwnerReference update configMap/secret object as owned by instance
func (r *ReconcileBOSHDeployment) updateObjectWithOwnerReference(ctx context.Context, instance *bdv1.BOSHDeployment, obj runtime.Object) error {
	// get the existing object meta
	metaObj, ok := obj.(metav1.Object)
	if !ok {
		return fmt.Errorf("%T does not implement metav1.Object interface", obj)
	}

	// retrieve the existing object
	key := client.ObjectKey{
		Name:      metaObj.GetName(),
		Namespace: metaObj.GetNamespace(),
	}
	err := r.client.Get(ctx, key, obj)
	if err != nil {
		return errors.Wrapf(err, "failed to get  secret '%s/%s'", metaObj.GetName(), metaObj.GetNamespace())
	}

	if err := r.setReference(instance, obj.(metav1.Object), r.scheme); err != nil {
		return errors.Wrap(err, "could not set reference for a secret for a BOSH Deployment")
	}

	// update the existing object
	err = r.client.Update(ctx, obj)
	if err != nil {
		return errors.Wrapf(err, "failed to update configMap '%s/%s'", metaObj.GetName(), metaObj.GetNamespace())
	}

	return nil
}

// setFinalizer Add the object's Finalizer and update if necessary
func (r *ReconcileBOSHDeployment) setFinalizer(ctx context.Context, instance *bdv1.BOSHDeployment) error {
	if !finalizer.HasFinalizer(instance) {
		log.Debugf(ctx, "Adding Finalizer to BoshDeployment '%s'", instance.GetName())
		// Fetch latest BoshDeployment before update
		key := types.NamespacedName{Namespace: instance.GetNamespace(), Name: instance.GetName()}
		err := r.client.Get(ctx, key, instance)
		if err != nil {
			return errors.Wrapf(err, "could not get BoshDeployment '%s'", instance.GetName())
		}

		finalizer.AddFinalizer(instance)

		err = r.client.Update(ctx, instance)
		if err != nil {
			log.Errorf(ctx, "Could not add finalizer from BoshDeployment '%s': %v", instance.GetName(), err)
			return err
		}
	}

	return nil
}

// handleDeletion remove all ownership from configs and the finalizer from instance
func (r *ReconcileBOSHDeployment) handleDeletion(ctx context.Context, instance *bdv1.BOSHDeployment) (reconcile.Result, error) {

	existingConfigs, err := r.owner.ListConfigsOwnedBy(ctx, instance)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "Could not list ConfigMaps and Secrets owned by '%s'", instance.Name)
	}
	err = r.owner.RemoveOwnerReferences(ctx, instance, existingConfigs)
	if err != nil {
		log.Errorf(ctx, "Could not remove OwnerReferences pointing to instance '%s': %s", instance.Name, err)
		return reconcile.Result{Requeue: true, RequeueAfter: 1 * time.Second}, err
	}

	// Use createOrUpdate pattern to remove finalizer
	_, err = controllerutil.CreateOrUpdate(ctx, r.client, instance.DeepCopy(), func(obj runtime.Object) error {
		exstInstance, ok := obj.(*bdv1.BOSHDeployment)
		if !ok {
			return fmt.Errorf("object is not a BOSHDeployment")
		}
		finalizer.RemoveFinalizer(exstInstance)
		return nil
	})
	if err != nil {
		log.Errorf(ctx, "Could not remove finalizer from BOSHDeployment '%s': %s", instance.GetName(), err)
		return reconcile.Result{Requeue: true, RequeueAfter: 1 * time.Second}, errors.Wrapf(err, "Could updating BOSHDeployment '%s'", instance.GetName())
	}

	return reconcile.Result{}, nil
}

// generateVariableSecrets create variables extendedSecrets
func (r *ReconcileBOSHDeployment) generateVariableSecrets(ctx context.Context, instance *bdv1.BOSHDeployment, manifest *bdm.Manifest, variables []esv1.ExtendedSecret) error {
	log.Debug(ctx, "Creating variables extendedSecrets")
	var err error
	for _, variable := range variables {
		// Set BOSHDeployment instance as the owner and controller
		if err := r.setReference(instance, &variable, r.scheme); err != nil {
			return errors.Wrap(err, "could not set reference for an ExtendedStatefulSet for a BOSH Deployment")
		}

		_, err = controllerutil.CreateOrUpdate(ctx, r.client, variable.DeepCopy(), func(obj runtime.Object) error {
			s, ok := obj.(*esv1.ExtendedSecret)
			if !ok {
				return fmt.Errorf("object is not an ExtendedSecret")
			}
			s.Spec = variable.Spec
			return nil
		})
		if err != nil {
			return errors.Wrapf(err, "creating or updating ExtendedSecret '%s'", variable.Name)
		}
	}

	instance.Status.State = VariableGeneratedState

	return nil
}

// createVariableInterpolationEJob create temp manifest and variable interpolation eJob
func (r *ReconcileBOSHDeployment) createVariableInterpolationEJob(ctx context.Context, instance *bdv1.BOSHDeployment, manifest *bdm.Manifest, varIntEJob *ejv1.ExtendedJob) error {

	// Create temp manifest as variable interpolation job input.
	// Ops files have been applied on this manifest.
	tempManifestBytes, err := yaml.Marshal(manifest)
	if err != nil {
		return errors.Wrap(err, "could not marshal temp manifest")
	}

	tempManifestSecretName := names.CalculateSecretName(names.DeploymentSecretTypeManifestWithOps, manifest.Name, "")

	// Create a secret object for the manifest
	tempManifestSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tempManifestSecretName,
			Namespace: instance.GetNamespace(),
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, r.client, tempManifestSecret, func(obj runtime.Object) error {
		s, ok := obj.(*corev1.Secret)
		if !ok {
			return fmt.Errorf("object is not a Secret")
		}
		s.Data = map[string][]byte{}
		s.StringData = map[string]string{
			"manifest.yaml": string(tempManifestBytes),
		}
		return nil
	})
	if err != nil {
		return errors.Wrapf(err, "creating or updating Secret '%s'", tempManifestSecret.Name)
	}

	log.Debug(ctx, "Creating variable interpolation extendedJob")
	// Set BOSHDeployment instance as the owner and controller
	if err := r.setReference(instance, varIntEJob, r.scheme); err != nil {
		log.WithEvent(instance, "NewJobForVariableInterpolationError").Errorf(ctx, "Failed to set ownerReference for ExtendedJob '%s': %v", varIntEJob.GetName(), err)
		return err
	}

	_, err = controllerutil.CreateOrUpdate(ctx, r.client, varIntEJob.DeepCopy(), func(obj runtime.Object) error {
		exstEJob, ok := obj.(*ejv1.ExtendedJob)
		if !ok {
			return fmt.Errorf("object is not an ExtendedJob")
		}

		exstEJob.Labels = varIntEJob.Labels
		exstEJob.Spec = varIntEJob.Spec
		return nil
	})
	if err != nil {
		log.WarningEvent(ctx, instance, "CreateExtendedJobForVariableInterpolationError", err.Error())
		return errors.Wrapf(err, "creating or updating ExtendedJob '%s'", varIntEJob.Name)
	}

	instance.Status.State = VariableInterpolatedState

	return nil
}

// createDataGatheringJob gather data from manifest
func (r *ReconcileBOSHDeployment) createDataGatheringJob(ctx context.Context, instance *bdv1.BOSHDeployment, dataGatheringEJob *ejv1.ExtendedJob) error {
	log.Debugf(ctx, "Creating data gathering extendedJob %s/%s", dataGatheringEJob.Namespace, dataGatheringEJob.Name)

	// Set BOSHDeployment instance as the owner and controller
	if err := r.setReference(instance, dataGatheringEJob, r.scheme); err != nil {
		log.WithEvent(instance, "NewJobForDataGatheringError").Errorf(ctx, "Failed to set ownerReference for ExtendedJob '%s': %v", dataGatheringEJob.GetName(), err)
		return err
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.client, dataGatheringEJob.DeepCopy(), func(obj runtime.Object) error {
		exstEJob, ok := obj.(*ejv1.ExtendedJob)
		if !ok {
			return fmt.Errorf("object is not an ExtendedJob")
		}

		exstEJob.Labels = dataGatheringEJob.Labels
		exstEJob.Spec = dataGatheringEJob.Spec
		return nil
	})
	if err != nil {
		log.WarningEvent(ctx, instance, "CreateJobForDataGatheringError", err.Error())
		return errors.Wrapf(err, "creating or updating ExtendedJob '%s'", dataGatheringEJob.Name)
	}

	instance.Status.State = DataGatheredState

	return nil
}

// createBPMConfigsJob creates an ejob for creating the BPM configs from manifest
func (r *ReconcileBOSHDeployment) createBPMConfigsJob(ctx context.Context, instance *bdv1.BOSHDeployment, bpmConfigsJob *ejv1.ExtendedJob) error {
	log.Debugf(ctx, "Creating BPM configs extendedJob %s/%s", bpmConfigsJob.Namespace, bpmConfigsJob.Name)

	// Set BOSHDeployment instance as the owner and controller
	if err := r.setReference(instance, bpmConfigsJob, r.scheme); err != nil {
		log.WithEvent(instance, "NewJobForDataGatheringError").Errorf(ctx, "Failed to set ownerReference for ExtendedJob '%s': %v", bpmConfigsJob.GetName(), err)
		return err
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.client, bpmConfigsJob.DeepCopy(), func(obj runtime.Object) error {
		exstEJob, ok := obj.(*ejv1.ExtendedJob)
		if !ok {
			return fmt.Errorf("object is not an ExtendedJob")
		}

		exstEJob.Labels = bpmConfigsJob.Labels
		exstEJob.Spec = bpmConfigsJob.Spec
		return nil
	})
	if err != nil {
		log.WarningEvent(ctx, instance, "CreateBPMConfigsJobError", err.Error())
		return errors.Wrapf(err, "creating or updating ExtendedJob '%s'", bpmConfigsJob.Name)
	}

	instance.Status.State = BPMConfigsCreatedState

	return nil
}

// waitForBPM checks to see if all BPM information is available and returns an error if it isn't
func (r *ReconcileBOSHDeployment) waitForBPM(ctx context.Context, deployment *bdv1.BOSHDeployment, manifest *bdm.Manifest, containers []corev1.Container) (map[string]bpm.Configs, error) {
	// TODO: this approach is not good enough, we need to reconcile and trigger on all of these secrets
	// TODO: these secrets could exist, but not be up to date - we have to make sure they exist for the appropriate version

	result := map[string]bpm.Configs{}

	for _, container := range containers {
		_, secretName := names.CalculateEJobOutputSecretPrefixAndName(
			names.DeploymentSecretBpmInformation,
			manifest.Name,
			container.Name,
			false,
		)

		log.Debugf(ctx, "Getting latest secret '%s'", secretName)
		secret, err := r.versionedSecretStore.Latest(ctx, deployment.Namespace, secretName)
		if err != nil && apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("secret %s/%s doesn't exist", deployment.Namespace, secretName)
		} else if err != nil {
			return nil, errors.Wrapf(err, "failed to retrieve bpm configs secret %s/%s", deployment.Namespace, secretName)
		}

		bpmConfigs := bpm.Configs{}
		err = yaml.Unmarshal(secret.Data["bpm.yaml"], &bpmConfigs)
		if err != nil {
			return nil, fmt.Errorf("couldn't unmarshal bpm configs from secret %s/%s", deployment.Namespace, secretName)
		}
		result[container.Name] = bpmConfigs
	}

	return result, nil
}

// deployInstanceGroups create ExtendedJobs and ExtendedStatefulSets
func (r *ReconcileBOSHDeployment) deployInstanceGroups(ctx context.Context, instance *bdv1.BOSHDeployment, kubeConfigs *bdm.KubeConfig) error {
	log.Debug(ctx, "Creating extendedJobs and extendedStatefulSets of instance groups")
	for _, eJob := range kubeConfigs.Errands {
		// Set BOSHDeployment instance as the owner and controller
		if err := r.setReference(instance, &eJob, r.scheme); err != nil {
			log.WarningEvent(ctx, instance, "NewExtendedJobForDeploymentError", err.Error())
			return errors.Wrap(err, "couldn't set reference for an ExtendedJob for a BOSH Deployment")
		}

		_, err := controllerutil.CreateOrUpdate(ctx, r.client, eJob.DeepCopy(), func(obj runtime.Object) error {
			exstEJob, ok := obj.(*ejv1.ExtendedJob)
			if !ok {
				return fmt.Errorf("object is not an ExtendedJob")
			}

			exstEJob.Labels = eJob.Labels
			exstEJob.Spec = eJob.Spec
			return nil
		})
		if err != nil {
			log.WarningEvent(ctx, instance, "CreateExtendedJobForDeploymentError", err.Error())
			return errors.Wrapf(err, "creating or updating ExtendedJob '%s'", eJob.Name)
		}
	}

	for _, svc := range kubeConfigs.Services {
		// Set BOSHDeployment instance as the owner and controller
		if err := r.setReference(instance, &svc, r.scheme); err != nil {
			log.WarningEvent(ctx, instance, "NewServiceForDeploymentError", err.Error())
			return errors.Wrap(err, "couldn't set reference for a Service for a BOSH Deployment")
		}

		_, err := controllerutil.CreateOrUpdate(ctx, r.client, svc.DeepCopy(), func(obj runtime.Object) error {
			exstSvc, ok := obj.(*corev1.Service)
			if !ok {
				return fmt.Errorf("object is not a Service")
			}

			exstSvc.Labels = svc.Labels
			svc.Spec.ClusterIP = exstSvc.Spec.ClusterIP
			exstSvc.Spec = svc.Spec
			return nil
		})
		if err != nil {
			log.WarningEvent(ctx, instance, "CreateServiceForDeploymentError", err.Error())
			return errors.Wrapf(err, "creating or updating Service '%s'", svc.Name)
		}
	}

	for _, eSts := range kubeConfigs.InstanceGroups {
		// Set BOSHDeployment instance as the owner and controller
		if err := r.setReference(instance, &eSts, r.scheme); err != nil {
			log.WarningEvent(ctx, instance, "NewExtendedStatefulSetForDeploymentError", err.Error())
			return errors.Wrap(err, "couldn't set reference for an ExtendedStatefulSet for a BOSH Deployment")
		}

		_, err := controllerutil.CreateOrUpdate(ctx, r.client, eSts.DeepCopy(), func(obj runtime.Object) error {
			exstSts, ok := obj.(*estsv1.ExtendedStatefulSet)
			if !ok {
				return fmt.Errorf("object is not an ExtendStatefulSet")
			}

			exstSts.Labels = eSts.Labels
			exstSts.Spec = eSts.Spec
			return nil
		})
		if err != nil {
			log.WarningEvent(ctx, instance, "CreateExtendedStatefulSetForDeploymentError", err.Error())
			return errors.Wrapf(err, "creating or updating ExtendedStatefulSet '%s'", eSts.Name)
		}
	}

	instance.Status.State = DeployingState

	return nil
}

// actionOnDeploying check out deployment status
func (r *ReconcileBOSHDeployment) actionOnDeploying(ctx context.Context, instance *bdv1.BOSHDeployment) error {
	// TODO Check deployment
	instance.Status.State = DeployedState

	return nil
}
