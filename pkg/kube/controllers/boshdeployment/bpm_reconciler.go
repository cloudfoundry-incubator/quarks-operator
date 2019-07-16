package boshdeployment

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/bpm"
	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	estsv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	log "code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
	vss "code.cloudfoundry.org/cf-operator/pkg/kube/util/versionedsecretstore"
)

type DesiredManifest interface {
	DesiredManifest(ctx context.Context, boshDeploymentName, namespace string) (*bdm.Manifest, error)
}

var _ reconcile.Reconciler = &ReconcileBOSHDeployment{}

// NewBPMReconciler returns a new reconcile.Reconciler
func NewBPMReconciler(ctx context.Context, config *config.Config, mgr manager.Manager, resolver DesiredManifest, srf setReferenceFunc, kubeConverter *bdm.KubeConverter) reconcile.Reconciler {
	return &ReconcileBPM{
		ctx:           ctx,
		config:        config,
		client:        mgr.GetClient(),
		scheme:        mgr.GetScheme(),
		resolver:      resolver,
		setReference:  srf,
		kubeConverter: kubeConverter,
	}
}

// ReconcileBPM reconciles an Instance Group BPM versioned secret
type ReconcileBPM struct {
	ctx           context.Context
	config        *config.Config
	client        client.Client
	scheme        *runtime.Scheme
	resolver      DesiredManifest
	setReference  setReferenceFunc
	kubeConverter *bdm.KubeConverter
}

// Reconcile reconciles an Instance Group BPM versioned secret read the corresponding
// desired manifest. It then applies BPM information and deploys instance groups.
func (r *ReconcileBPM) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Set the ctx to be Background, as the top-level context for incoming requests.
	ctx, cancel := context.WithTimeout(r.ctx, r.config.CtxTimeOut)
	defer cancel()

	log.Infof(ctx, "Reconciling Instance Group BPM versioned secret '%s'", request.NamespacedName)
	bpmSecret := &corev1.Secret{}
	err := r.client.Get(ctx, request.NamespacedName, bpmSecret)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Debug(ctx, "Skip reconcile: Instance Group BPM versioned secret not found")
			return reconcile.Result{}, nil
		}

		// Error reading the object - requeue the request.
		return reconcile.Result{
				Requeue:      true,
				RequeueAfter: time.Second * 5,
			},
			log.WithEvent(bpmSecret, "GetBPMSecret").Errorf(ctx, "Failed to get Instance Group BPM versioned secret '%s': %v", request.NamespacedName, err)
	}

	// Get the label from the BPM Secret and read the corresponding desired manifest
	var boshDeploymentName string
	var ok bool
	if boshDeploymentName, ok = bpmSecret.Labels[bdv1.LabelDeploymentName]; !ok {
		return reconcile.Result{},
			log.WithEvent(bpmSecret, "GetBOSHDeploymentLabel").Errorf(ctx, "There's no label for a BOSH Deployment name on the Instance Group BPM versioned bpmSecret '%s'", request.NamespacedName)
	}
	manifest, err := r.resolver.DesiredManifest(ctx, boshDeploymentName, request.Namespace)
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(bpmSecret, "DesiredManifestReadError").Errorf(ctx, "Failed to read desired manifest '%s': %v", request.NamespacedName, err)
	}

	// Apply BPM information
	instanceGroupName, ok := bpmSecret.Labels[ejv1.LabelInstanceGroup]
	if !ok {
		return reconcile.Result{},
			log.WithEvent(bpmSecret, "LabelMissingError").Errorf(ctx, "Missing container label for bpm information bpmSecret '%s'", request.NamespacedName)
	}

	resources, err := r.applyBPMResources(bpmSecret, manifest)
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(bpmSecret, "BPMApplyingError").Errorf(ctx, "Failed to apply BPM information: %v", err)
	}

	// Start the instance groups referenced by this BPM secret
	instanceName, ok := bpmSecret.Labels[bdv1.LabelDeploymentName]
	if !ok {
		return reconcile.Result{},
			log.WithEvent(bpmSecret, "LabelMissingError").Errorf(ctx, "Missing deployment mame label for bpm information bpmSecret '%s'", request.NamespacedName)
	}

	// Deploy instance groups
	instance := &bdv1.BOSHDeployment{}
	err = r.client.Get(ctx, types.NamespacedName{Namespace: request.Namespace, Name: instanceName}, instance)
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(bpmSecret, "GetBOSHDeployment").Errorf(ctx, "Failed to get BoshDeployment instance '%s': %v", instanceName, err)
	}

	err = r.deployInstanceGroups(ctx, instance, instanceGroupName, resources)
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(bpmSecret, "InstanceGroupStartError").Errorf(ctx, "Failed to start : %v", err)
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileBPM) applyBPMResources(bpmSecret *corev1.Secret, manifest *bdm.Manifest) (*bdm.BPMResources, error) {
	resources := &bdm.BPMResources{}
	bpmConfigs := bpm.Configs{}

	instanceGroupName, ok := bpmSecret.Labels[ejv1.LabelInstanceGroup]
	if !ok {
		return resources, errors.Errorf("Missing container label for bpm information secret '%s'", bpmSecret.Name)
	}

	version, ok := bpmSecret.Labels[vss.LabelVersion]
	if !ok {
		return resources, errors.Errorf("Missing version label for bpm information secret '%s'", bpmSecret.Name)
	}

	if val, ok := bpmSecret.Data["bpm.yaml"]; ok {
		err := yaml.Unmarshal(val, &bpmConfigs)
		if err != nil {
			return resources, err
		}
	} else {
		return resources, errors.New("Couldn't find bpm.yaml key in manifest secret")
	}

	instanceGroup := &bdm.InstanceGroup{}
	for _, ig := range manifest.InstanceGroups {
		if ig.Name == instanceGroupName {
			instanceGroup = ig
			break
		}
	}

	resources, err := r.kubeConverter.BPMResources(manifest.Name, version, instanceGroup, manifest, bpmConfigs)
	if err != nil {
		return resources, err
	}

	return resources, nil
}

// deployInstanceGroups create or update ExtendedJobs and ExtendedStatefulSets for instance groups
func (r *ReconcileBPM) deployInstanceGroups(ctx context.Context, instance *bdv1.BOSHDeployment, instanceGroupName string, resources *bdm.BPMResources) error {
	log.Debugf(ctx, "Creating extendedJobs and extendedStatefulSets for instance group '%s'", instanceGroupName)

	for _, eJob := range resources.Errands {
		if eJob.Labels[bdm.LabelInstanceGroupName] != instanceGroupName {
			continue
		}

		if err := r.setReference(instance, &eJob, r.scheme); err != nil {
			return log.WithEvent(instance, "ExtendedJobForDeploymentError").Errorf(ctx, "Failed to set reference for ExtendedJob instance group '%s' : %v", instanceGroupName, err)
		}

		op, err := controllerutil.CreateOrUpdate(ctx, r.client, eJob.DeepCopy(), func(obj runtime.Object) error {
			if existingEJob, ok := obj.(*ejv1.ExtendedJob); ok {
				eJob.ObjectMeta.ResourceVersion = existingEJob.ObjectMeta.ResourceVersion
				eJob.DeepCopyInto(existingEJob)

				return nil
			}
			return fmt.Errorf("object is not an ExtendedJob")
		})
		if err != nil {
			return log.WithEvent(instance, "ApplyExtendedJobError").Errorf(ctx, "Failed to apply ExtendedJob for instance group '%s' : %v", instanceGroupName, err)
		}

		log.Debugf(ctx, "ExtendedJob '%s' has been %s", eJob.Name, op)
	}

	for _, svc := range resources.Services {
		if svc.Labels[bdm.LabelInstanceGroupName] != instanceGroupName {
			continue
		}

		if err := r.setReference(instance, &svc, r.scheme); err != nil {
			return log.WithEvent(instance, "ServiceForDeploymentError").Errorf(ctx, "Failed to set reference for Service instance group '%s' : %v", instanceGroupName, err)
		}

		op, err := controllerutil.CreateOrUpdate(ctx, r.client, svc.DeepCopy(), func(obj runtime.Object) error {
			if existingSvc, ok := obj.(*corev1.Service); ok {
				// Should keep current ClusterIP and ResourceVersion when update
				svc.Spec.ClusterIP = existingSvc.Spec.ClusterIP
				svc.ObjectMeta.ResourceVersion = existingSvc.ObjectMeta.ResourceVersion
				svc.DeepCopyInto(existingSvc)
				return nil
			}
			return fmt.Errorf("object is not a Service")
		})
		if err != nil {
			return log.WithEvent(instance, "ApplyServiceError").Errorf(ctx, "Failed to apply Service for instance group '%s' : %v", instanceGroupName, err)
		}

		log.Debugf(ctx, "Service '%s' has been %s", svc.Name, op)
	}

	// Create a persistent volume claims for containers of the instance group
	// Right now, only one pvc is being created at /var/vcap/store
	for _, disk := range resources.Disks {
		if disk.PersistentVolumeClaim != nil {
			err := r.createPersistentVolumeClaim(ctx, disk.PersistentVolumeClaim)
			if err != nil {
				return log.WithEvent(instance, "ApplyPersistentVolumeClaimError").Errorf(ctx, "Failed to apply PersistentVolumeClaim for instance group '%s' : %v", instanceGroupName, err)
			}
		}
	}

	for _, eSts := range resources.InstanceGroups {
		if eSts.Labels[bdm.LabelInstanceGroupName] != instanceGroupName {
			continue
		}

		if err := r.setReference(instance, &eSts, r.scheme); err != nil {
			return log.WithEvent(instance, "ExtendedStatefulSetForDeploymentError").Errorf(ctx, "Failed to set reference for ExtendedStatefulSet instance group '%s' : %v", instanceGroupName, err)
		}

		op, err := controllerutil.CreateOrUpdate(ctx, r.client, eSts.DeepCopy(), func(obj runtime.Object) error {
			if existingSts, ok := obj.(*estsv1.ExtendedStatefulSet); ok {
				eSts.ObjectMeta.ResourceVersion = existingSts.ObjectMeta.ResourceVersion
				eSts.DeepCopyInto(existingSts)

				return nil
			}
			return fmt.Errorf("object is not an ExtendStatefulSet")
		})
		if err != nil {
			return log.WithEvent(instance, "ApplyExtendedStatefulSetError").Errorf(ctx, "Failed to apply ExtendedStatefulSet for instance group '%s' : %v", instanceGroupName, err)
		}

		log.Debugf(ctx, "ExtendStatefulSet '%s' has been %s", eSts.Name, op)
	}

	return nil
}

func (r *ReconcileBPM) createPersistentVolumeClaim(ctx context.Context, persistentVolumeClaim *corev1.PersistentVolumeClaim) error {
	log.Debugf(ctx, "Creating persistentVolumeClaim '%s'", persistentVolumeClaim.Name)

	// Create only if PVC doesn't not exist
	err := r.client.Create(ctx, persistentVolumeClaim)
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			log.Debugf(ctx, "Skip creating PersistentVolumeClaim '%s' because it already exists", persistentVolumeClaim.Name)
			return nil
		}
		return err

	}

	return nil
}
