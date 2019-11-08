package boshdeployment

import (
	"context"
	"strconv"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/bpm"
	"code.cloudfoundry.org/cf-operator/pkg/bosh/converter"
	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	esv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedsecret/v1alpha1"
	essv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	exsts "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/extendedstatefulset"
	qsv1a1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/quarkssecret/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/mutate"
	ejv1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/extendedjob/v1alpha1"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	log "code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	"code.cloudfoundry.org/quarks-utils/pkg/meltdown"
	"code.cloudfoundry.org/quarks-utils/pkg/names"
	"code.cloudfoundry.org/quarks-utils/pkg/versionedsecretstore"
)

// DesiredManifest unmarshals desired manifest from the manifest secret
type DesiredManifest interface {
	DesiredManifest(ctx context.Context, boshDeploymentName, namespace string) (*bdm.Manifest, error)
}

// KubeConverter converts k8s resources from single BOSH manifest
type KubeConverter interface {
	BPMResources(manifestName string, dns manifest.DomainNameService, exstsVersion string, instanceGroup *bdm.InstanceGroup, releaseImageProvider converter.ReleaseImageProvider, bpmConfigs bpm.Configs, igResolvedSecretVersion string) (*converter.BPMResources, error)
	Variables(manifestName string, variables []bdm.Variable) ([]esv1.ExtendedSecret, error)
}

var _ reconcile.Reconciler = &ReconcileBOSHDeployment{}

// NewBPMReconciler returns a new reconcile.Reconciler
func NewBPMReconciler(ctx context.Context, config *config.Config, mgr manager.Manager, resolver DesiredManifest, srf setReferenceFunc, kubeConverter KubeConverter) reconcile.Reconciler {
	return &ReconcileBPM{
		ctx:                  ctx,
		config:               config,
		client:               mgr.GetClient(),
		scheme:               mgr.GetScheme(),
		resolver:             resolver,
		setReference:         srf,
		kubeConverter:        kubeConverter,
		versionedSecretStore: versionedsecretstore.NewVersionedSecretStore(mgr.GetClient()),
	}
}

// ReconcileBPM reconciles an Instance Group BPM versioned secret
type ReconcileBPM struct {
	ctx                  context.Context
	config               *config.Config
	client               client.Client
	scheme               *runtime.Scheme
	resolver             DesiredManifest
	setReference         setReferenceFunc
	kubeConverter        KubeConverter
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
		log.WithEvent(bpmSecret, "GetBPMSecret").Errorf(ctx, "Failed to get Instance Group BPM versioned secret '%s': %v", request.NamespacedName, err)
		return reconcile.Result{RequeueAfter: time.Second * 5}, nil
	}

	if meltdown.NewAnnotationWindow(r.config.MeltdownDuration, bpmSecret.ObjectMeta.Annotations).Contains(time.Now()) {
		log.WithEvent(bpmSecret, "Meltdown").Debugf(ctx, "Resource '%s' is in meltdown, requeue reconcile after %s", bpmSecret.Name, r.config.MeltdownRequeueAfter)
		return reconcile.Result{RequeueAfter: r.config.MeltdownRequeueAfter}, nil
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

	// Start the instance groups referenced by this BPM secret
	instanceName, ok := bpmSecret.Labels[bdv1.LabelDeploymentName]
	if !ok {
		return reconcile.Result{},
			log.WithEvent(bpmSecret, "LabelMissingError").Errorf(ctx, "Missing deployment mame label for bpm information bpmSecret '%s'", request.NamespacedName)
	}

	instance := &bdv1.BOSHDeployment{}
	err = r.client.Get(ctx, types.NamespacedName{Namespace: request.Namespace, Name: instanceName}, instance)
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(bpmSecret, "GetBOSHDeployment").Errorf(ctx, "Failed to get BoshDeployment instance '%s': %v", instanceName, err)
	}

	err = manifest.DNS.Reconcile(ctx, request.Namespace, r.client, func(object v1.Object) error {
		return r.setReference(instance, object, r.scheme)
	})

	if err != nil {
		return reconcile.Result{},
			log.WithEvent(bpmSecret, "DnsReconcileError").Errorf(ctx, "Failed to reconcile dns: %v", err)
	}

	resources, err := r.applyBPMResources(bpmSecret, manifest)
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(bpmSecret, "BPMApplyingError").Errorf(ctx, "Failed to apply BPM information: %v", err)
	}

	if resources == nil {
		log.WithEvent(bpmSecret, "SkipReconcile").Infof(ctx, "Skip reconcile: BPM resources not found")
		return reconcile.Result{}, nil
	}

	// Deploy instance groups
	err = r.deployInstanceGroups(ctx, instance, instanceGroupName, resources)
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(bpmSecret, "InstanceGroupStartError").Errorf(ctx, "Failed to start: %v", err)
	}

	meltdown.SetLastReconcile(&bpmSecret.ObjectMeta, time.Now())
	err = r.client.Update(ctx, bpmSecret)
	if err != nil {
		log.WithEvent(bpmSecret, "UpdateError").Errorf(ctx, "Failed to update reconcile timestamp on BPM versioned secret '%s' (%v): %s", bpmSecret.Name, bpmSecret.ResourceVersion, err)
		return reconcile.Result{Requeue: false}, nil
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileBPM) applyBPMResources(bpmSecret *corev1.Secret, manifest *bdm.Manifest) (*converter.BPMResources, error) {

	instanceGroupName, ok := bpmSecret.Labels[ejv1.LabelInstanceGroup]
	if !ok {
		return nil, errors.Errorf("Missing container label for bpm information secret '%s'", bpmSecret.Name)
	}

	var bpmInfo bdm.BPMInfo
	if val, ok := bpmSecret.Data["bpm.yaml"]; ok {
		err := yaml.Unmarshal(val, &bpmInfo)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, errors.New("Couldn't find bpm.yaml key in manifest secret")
	}

	instanceGroup, found := manifest.InstanceGroups.InstanceGroupByName(instanceGroupName)
	if !found {
		return nil, errors.Errorf("instance group '%s' not found", instanceGroupName)
	}

	// Fetch exsts version
	exstendedStatefulSet := &essv1.ExtendedStatefulSet{}
	exstendedStatefulSetName := instanceGroup.ExtendedStatefulsetName(manifest.Name)
	err := r.client.Get(r.ctx, types.NamespacedName{Namespace: r.config.Namespace, Name: exstendedStatefulSetName}, exstendedStatefulSet)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, errors.Errorf("Failed to get ExtendedStatefulSet instance '%s': %v", exstendedStatefulSetName, err)
		}
	}
	_, exstsVersion, err := exsts.GetMaxStatefulSetVersion(r.ctx, r.client, exstendedStatefulSet)
	if err != nil {
		return nil, err
	}
	exstsVersion = exstsVersion + 1
	exstsVersionString := strconv.Itoa(exstsVersion)
	if err != nil {
		return nil, err
	}

	// Fetch ig resolved secret version
	igResolvedSecretName := names.CalculateIGSecretName(
		names.DeploymentSecretTypeInstanceGroupResolvedProperties,
		manifest.Name,
		instanceGroupName,
		"",
	)
	igResolvedSecret, err := r.versionedSecretStore.Latest(r.ctx, r.config.Namespace, igResolvedSecretName)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read latest versioned secret %s for bosh deployment %s", igResolvedSecretName, manifest.Name)
	}
	igResolvedSecretVersion := igResolvedSecret.GetLabels()[versionedsecretstore.LabelVersion]

	resources, err := r.kubeConverter.BPMResources(manifest.Name, manifest.DNS, exstsVersionString, instanceGroup, manifest, bpmInfo.Configs, igResolvedSecretVersion)
	if err != nil {
		return resources, err
	}

	return resources, nil
}

// deployInstanceGroups create or update ExtendedJobs and ExtendedStatefulSets for instance groups
func (r *ReconcileBPM) deployInstanceGroups(ctx context.Context, instance *bdv1.BOSHDeployment, instanceGroupName string, resources *converter.BPMResources) error {
	log.Debugf(ctx, "Creating quarksJobs and quarksStatefulSets for instance group '%s'", instanceGroupName)

	for _, eJob := range resources.Errands {
		if eJob.Labels[bdm.LabelInstanceGroupName] != instanceGroupName {
			log.Debugf(ctx, "Skipping EJob definition '%s' for instance group '%s' because of missmatching '%s' label", eJob.Name, instance.Name, bdm.LabelInstanceGroupName)
			continue
		}

		if err := r.setReference(instance, &eJob, r.scheme); err != nil {
			return log.WithEvent(instance, "ExtendedJobForDeploymentError").Errorf(ctx, "Failed to set reference for ExtendedJob instance group '%s' : %v", instanceGroupName, err)
		}

		op, err := controllerutil.CreateOrUpdate(ctx, r.client, &eJob, mutate.EJobMutateFn(&eJob))
		if err != nil {
			return log.WithEvent(instance, "ApplyExtendedJobError").Errorf(ctx, "Failed to apply ExtendedJob for instance group '%s' : %v", instanceGroupName, err)
		}

		log.Debugf(ctx, "ExtendedJob '%s' has been %s", eJob.Name, op)
	}

	for _, svc := range resources.Services {
		if svc.Labels[bdm.LabelInstanceGroupName] != instanceGroupName {
			log.Debugf(ctx, "Skipping Service definition '%s' for instance group '%s' because of missmatching '%s' label", svc.Name, instance.Name, bdm.LabelInstanceGroupName)
			continue
		}

		if err := r.setReference(instance, &svc, r.scheme); err != nil {
			return log.WithEvent(instance, "ServiceForDeploymentError").Errorf(ctx, "Failed to set reference for Service instance group '%s' : %v", instanceGroupName, err)
		}

		op, err := controllerutil.CreateOrUpdate(ctx, r.client, &svc, mutate.ServiceMutateFn(&svc))
		if err != nil {
			return log.WithEvent(instance, "ApplyServiceError").Errorf(ctx, "Failed to apply Service for instance group '%s' : %v", instanceGroupName, err)
		}

		log.Debugf(ctx, "Service '%s' has been %s", svc.Name, op)
	}

	for _, eSts := range resources.InstanceGroups {
		if eSts.Labels[bdm.LabelInstanceGroupName] != instanceGroupName {
			log.Debugf(ctx, "Skipping ESts definition '%s' for instance group '%s' because of missmatching '%s' label", eSts.Name, instance.Name, bdm.LabelInstanceGroupName)
			continue
		}

		if err := r.setReference(instance, &eSts, r.scheme); err != nil {
			return log.WithEvent(instance, "ExtendedStatefulSetForDeploymentError").Errorf(ctx, "Failed to set reference for QuarksStatefulSet instance group '%s' : %v", instanceGroupName, err)
		}

		op, err := controllerutil.CreateOrUpdate(ctx, r.client, &eSts, mutate.EStsMutateFn(&eSts))
		if err != nil {
			return log.WithEvent(instance, "ApplyExtendedStatefulSetError").Errorf(ctx, "Failed to apply QuarksStatefulSet for instance group '%s' : %v", instanceGroupName, err)
		}

		log.Debugf(ctx, "ExtendStatefulSet '%s' has been %s", eSts.Name, op)
	}

	return nil
}
