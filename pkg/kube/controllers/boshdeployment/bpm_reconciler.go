package boshdeployment

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	bpm "code.cloudfoundry.org/cf-operator/pkg/bosh/bpm"
	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	estsv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	log "code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/names"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/owner"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/versionedsecretstore"
)

var _ reconcile.Reconciler = &ReconcileBOSHDeployment{}

// NewBPMReconciler returns a new reconcile.Reconciler
func NewBPMReconciler(ctx context.Context, config *config.Config, mgr manager.Manager, resolver bdm.Resolver, srf setReferenceFunc) reconcile.Reconciler {
	versionedSecretStore := versionedsecretstore.NewVersionedSecretStore(mgr.GetClient())

	return &ReconcileBPM{
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

// ReconcileBPM reconciles a BOSHDeployment object
type ReconcileBPM struct {
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
func (r *ReconcileBPM) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Set the ctx to be Background, as the top-level context for incoming requests.
	ctx, cancel := context.WithTimeout(r.ctx, r.config.CtxTimeOut)
	defer cancel()

	log.Infof(ctx, "Reconciling BOSHDeployment %s", request.NamespacedName)
	instance := &bdv1.BOSHDeployment{}
	err := r.client.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Debug(ctx, "Skip reconcile: BOSHDeployment not found")
			return reconcile.Result{}, nil
		}
		err = log.WithEvent(instance, "GetBOSHDeploymentError").Errorf(ctx, "Failed to get BOSHDeployment '%s': %v", request.NamespacedName, err)
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Get state from instance
	instanceState := instance.Status.State

	// Resolve the manifest (incl. ops files and implicit variables)
	manifest, err := r.resolveManifest(ctx, instance)
	if err != nil {
		log.WithEvent(instance, "BadManifestError").Infof(ctx, "Error resolving the manifest %s", instance.GetName())
		return reconcile.Result{}, errors.Wrap(err, "could not resolve manifest")
	}

	// Generate all the kube objects we need for the manifest
	log.Debug(ctx, "Converting bosh manifest to kube objects")
	kubeConfigs := bdm.NewKubeConfig(r.config.Namespace, manifest)
	err = kubeConfigs.Convert(*manifest)
	if err != nil {
		err = log.WithEvent(instance, "BadManifestError").Errorf(ctx, "Error converting bosh manifest %s to kube objects: %s", manifest.Name, err)
		return reconcile.Result{}, err
	}

	switch instanceState {
	case BPMConfigsCreatedState:
		// Wait for all instance group property outputs to be ready
		// We need BPM information to start everything up
		bpmInfo, err := r.waitForBPM(ctx, instance, manifest, kubeConfigs)
		if err != nil {
			err = log.WithEvent(instance, "BPMInfoError").Errorf(ctx, "Waiting for BPM: %s", err)
			return reconcile.Result{Requeue: true, RequeueAfter: 5 * time.Second}, err
		}

		err = kubeConfigs.ApplyBPMInfo(bpmInfo)
		if err != nil {
			err = log.WithEvent(instance, "BPMApplyingError").Errorf(ctx, "Failed to apply BPM information: %s", err)
			return reconcile.Result{}, err
		}

		err = r.deployInstanceGroups(ctx, instance, kubeConfigs)
		if err != nil {
			err = log.WithEvent(instance, "InstanceGroupsError").Errorf(ctx, "Failed to deploy instance groups: %s", err)
			return reconcile.Result{}, err
		}

	default:
		return reconcile.Result{}, nil
	}

	log.Debugf(ctx, "Requeue the reconcile: BoshDeployment '%s/%s' is in state '%s'", instance.GetNamespace(), instance.GetName(), instance.Status.State)
	return reconcile.Result{Requeue: true}, r.updateInstanceState(ctx, instance)
}

// updateInstanceState update instance state
func (r *ReconcileBPM) updateInstanceState(ctx context.Context, currentInstance *bdv1.BOSHDeployment) error {
	currentManifestSHA1, _ := currentInstance.GetAnnotations()[bdv1.AnnotationManifestSHA1]

	// Fetch latest BOSHDeployment before update
	foundInstance := &bdv1.BOSHDeployment{}
	key := types.NamespacedName{Namespace: currentInstance.GetNamespace(), Name: currentInstance.GetName()}
	err := r.client.Get(ctx, key, foundInstance)
	if err != nil {
		log.Errorf(ctx, "Failed to get BOSHDeployment instance '%s': %v", currentInstance.GetName(), err)
		return errors.Wrapf(err, "could not get BOSHDeployment instance '%s' when update instance state", currentInstance.GetName())
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
			return errors.Wrapf(err, "could not update BOSHDeployment instance '%s' when update instance state", currentInstance.GetName())
		}
	}

	return nil
}

// resolveManifest resolves the manifest and applies ops files and implicit variable interpolation
func (r *ReconcileBPM) resolveManifest(ctx context.Context, instance *bdv1.BOSHDeployment) (*bdm.Manifest, error) {
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

// waitForBPM checks to see if all BPM information is available and returns an error if it isn't
func (r *ReconcileBPM) waitForBPM(ctx context.Context, deployment *bdv1.BOSHDeployment, manifest *bdm.Manifest, kubeConfigs *bdm.KubeConfig) (map[string]bpm.Configs, error) {
	// TODO: this approach is not good enough, we need to reconcile and trigger on all of these secrets
	// TODO: these secrets could exist, but not be up to date - we have to make sure they exist for the appropriate version

	result := map[string]bpm.Configs{}

	for _, container := range kubeConfigs.DataGatheringJob.Spec.Template.Spec.Containers {
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
func (r *ReconcileBPM) deployInstanceGroups(ctx context.Context, instance *bdv1.BOSHDeployment, kubeConfigs *bdm.KubeConfig) error {
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
