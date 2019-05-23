package boshdeployment

import (
	"context"
	"fmt"
	"time"

	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
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
	manifest, err := r.resolver.ReadDesiredManifest(ctx, boshDeploymentName, request.Namespace)
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(bpmSecret, "DesiredManifestReadError").Errorf(ctx, "Failed to read desired manifest '%s': %v", request.NamespacedName, err)
	}

	// Convert the manifest to kube objects
	log.Debug(ctx, "Converting bosh manifest to kube objects")
	kubeConfigs := bdm.NewKubeConfig(r.config.Namespace, manifest)
	err = kubeConfigs.Convert(*manifest)
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(bpmSecret, "ManifestConversionError").Errorf(ctx, "Failed to convert bosh manifest '%s' to kube objects: %v", request.NamespacedName, err)
	}

	// Apply BPM information
	bpmInfo := map[string]bpm.Configs{}
	instanceGroupName, ok := bpmSecret.Labels[ejv1.LabelPersistentSecretContainer]
	if !ok {
		return reconcile.Result{},
			log.WithEvent(bpmSecret, "LabelMissingError").Errorf(ctx, "Missing container label for bpm information bpmSecret '%s'", request.NamespacedName)
	}
	bpmConfigs := bpm.Configs{}
	err = yaml.Unmarshal(bpmSecret.Data["bpm.yaml"], &bpmConfigs)
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(bpmSecret, "BPMUnmarshalError").Errorf(ctx, "Couldn't unmarshal BPM configs from secret '%s'", request.NamespacedName)
	}
	bpmInfo[instanceGroupName] = bpmConfigs
	err = kubeConfigs.ApplyBPMInfo(bpmInfo)
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(bpmSecret, "BPMApplyingError").Errorf(ctx, "Failed to apply BPM information: %v", err)
	}

	// Start the instance groups referenced by this BPM secret
	err = r.deployInstanceGroups(ctx, bpmSecret, instanceGroupName, kubeConfigs)
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(bpmSecret, "InstanceGroupStartError").Errorf(ctx, "Failed to start : %v", err)
	}

	return reconcile.Result{}, nil
}

// deployInstanceGroups create or update ExtendedJobs and ExtendedStatefulSets for instance groups
func (r *ReconcileBPM) deployInstanceGroups(ctx context.Context, instance *corev1.Secret, instanceGroupName string, kubeConfigs *bdm.KubeConfig) error {
	log.Debugf(ctx, "Creating extendedJobs and extendedStatefulSets for instance group '%s'", instanceGroupName)

	for _, eJob := range kubeConfigs.Errands {
		if eJob.Labels[bdm.LabelInstanceGroupName] != instanceGroupName {
			continue
		}

		if err := r.setReference(instance, &eJob, r.scheme); err != nil {
			return log.WithEvent(instance, "ExtendedJobForDeploymentError").Errorf(ctx, "Failed to set reference for ExtendedJob instance group '%s' : %v", instanceGroupName, err)
		}

		_, err := controllerutil.CreateOrUpdate(ctx, r.client, eJob.DeepCopy(), func(obj runtime.Object) error {
			if existingEJob, ok := obj.(*ejv1.ExtendedJob); ok {
				eJob.DeepCopyInto(existingEJob)
				return nil
			}
			return fmt.Errorf("object is not an ExtendedJob")
		})
		if err != nil {
			return log.WithEvent(instance, "ApplyExtendedJobError").Errorf(ctx, "Failed to apply ExtendedJob for instance group '%s' : %v", instanceGroupName, err)
		}
	}

	for _, svc := range kubeConfigs.Services {
		if svc.Labels[bdm.LabelInstanceGroupName] != instanceGroupName {
			continue
		}

		if err := r.setReference(instance, &svc, r.scheme); err != nil {
			return log.WithEvent(instance, "ServiceForDeploymentError").Errorf(ctx, "Failed to set reference for Service instance group '%s' : %v", instanceGroupName, err)
		}

		_, err := controllerutil.CreateOrUpdate(ctx, r.client, svc.DeepCopy(), func(obj runtime.Object) error {
			if existingSvc, ok := obj.(*corev1.Service); ok {
				// Should keep current ClusterIP when update
				svc.Spec.ClusterIP = existingSvc.Spec.ClusterIP
				svc.DeepCopyInto(existingSvc)
				return nil
			}
			return fmt.Errorf("object is not a Service")
		})
		if err != nil {
			return log.WithEvent(instance, "ApplyServiceError").Errorf(ctx, "Failed to apply Service for instance group '%s' : %v", instanceGroupName, err)
		}
	}

	for _, eSts := range kubeConfigs.InstanceGroups {
		if eSts.Labels[bdm.LabelInstanceGroupName] != instanceGroupName {
			continue
		}

		if err := r.setReference(instance, &eSts, r.scheme); err != nil {
			return log.WithEvent(instance, "ExtendedStatefulSetForDeploymentError").Errorf(ctx, "Failed to set reference for ExtendedStatefulSet instance group '%s' : %v", instanceGroupName, err)
		}

		_, err := controllerutil.CreateOrUpdate(ctx, r.client, eSts.DeepCopy(), func(obj runtime.Object) error {
			if existingSts, ok := obj.(*estsv1.ExtendedStatefulSet); ok {
				eSts.DeepCopyInto(existingSts)
				return nil
			}
			return fmt.Errorf("object is not an ExtendStatefulSet")
		})
		if err != nil {
			return log.WithEvent(instance, "ApplyExtendedStatefulSetError").Errorf(ctx, "Failed to apply ExtendedStatefulSet for instance group '%s' : %v", instanceGroupName, err)
		}
	}

	return nil
}
