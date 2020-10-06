package boshdeployment

import (
	"context"
	"time"

	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	qjv1a1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/quarksjob/v1alpha1"
	"code.cloudfoundry.org/quarks-operator/pkg/bosh/converter"
	bdm "code.cloudfoundry.org/quarks-operator/pkg/bosh/manifest"
	bdv1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/mutate"
	qsv1a1 "code.cloudfoundry.org/quarks-secret/pkg/kube/apis/quarkssecret/v1alpha1"
	mutateqs "code.cloudfoundry.org/quarks-secret/pkg/kube/util/mutate"
	qstsv1a1 "code.cloudfoundry.org/quarks-statefulset/pkg/kube/apis/quarksstatefulset/v1alpha1"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	log "code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	"code.cloudfoundry.org/quarks-utils/pkg/logger"
	"code.cloudfoundry.org/quarks-utils/pkg/meltdown"
	"code.cloudfoundry.org/quarks-utils/pkg/pointers"
)

// BDPLStateCreating is the Bosh Deployment Status spec Creating State
const BDPLStateCreating = "Creating/Updating"

// JobFactory creates Jobs for a given manifest
type JobFactory interface {
	InstanceGroupManifestJob(namespace string, deploymentName string, manifest bdm.Manifest, linkInfos converter.LinkInfos, initialRollout bool) (*qjv1a1.QuarksJob, error)
}

// VariablesConverter converts BOSH variables into QuarksSecrets
type VariablesConverter interface {
	Variables(namespace string, manifestName string, variables []bdm.Variable) ([]qsv1a1.QuarksSecret, error)
}

// WithOps interpolates BOSH manifests and operations files to create the WithOps manifest
type WithOps interface {
	Manifest(ctx context.Context, bdpl *bdv1.BOSHDeployment, namespace string) (*bdm.Manifest, []string, error)
}

// Check that ReconcileBOSHDeployment implements the reconcile.Reconciler interface
var _ reconcile.Reconciler = &ReconcileBOSHDeployment{}

// ReconcileSkipDuration is the duration of merging consecutive triggers.
const ReconcileSkipDuration = 10 * time.Second

type setReferenceFunc func(owner, object metav1.Object, scheme *runtime.Scheme) error

// NewDeploymentReconciler returns a new reconcile.Reconciler
func NewDeploymentReconciler(ctx context.Context, config *config.Config, mgr manager.Manager, withops WithOps, jobFactory JobFactory, converter VariablesConverter, srf setReferenceFunc) reconcile.Reconciler {

	return &ReconcileBOSHDeployment{
		ctx:          ctx,
		config:       config,
		client:       mgr.GetClient(),
		scheme:       mgr.GetScheme(),
		withops:      withops,
		setReference: srf,
		jobFactory:   jobFactory,
		converter:    converter,
	}
}

// ReconcileBOSHDeployment reconciles a BOSHDeployment object
type ReconcileBOSHDeployment struct {
	ctx          context.Context
	config       *config.Config
	client       client.Client
	scheme       *runtime.Scheme
	withops      WithOps
	setReference setReferenceFunc
	jobFactory   JobFactory
	converter    VariablesConverter
}

// Reconcile starts the deployment process for a BOSHDeployment and deploys QuarksJobs to generate required properties for instance groups and rendered BPM
func (r *ReconcileBOSHDeployment) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the BOSHDeployment instance
	bdpl := &bdv1.BOSHDeployment{}

	// Set the ctx to be Background, as the top-level context for incoming requests.
	ctx, cancel := context.WithTimeout(r.ctx, r.config.CtxTimeOut)
	defer cancel()

	log.Infof(ctx, "Reconciling BOSHDeployment '%s'", request.NamespacedName)
	err := r.client.Get(ctx, request.NamespacedName, bdpl)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Debug(ctx, "Skip reconcile: BOSHDeployment not found")
			return reconcile.Result{}, nil
		}

		return reconcile.Result{},
			log.WithEvent(bdpl, "GetBOSHDeploymentError").Errorf(ctx, "failed to get BOSHDeployment '%s': %v", request.NamespacedName, err)
	}

	if bdpl.Status.LastReconcile == nil {
		now := metav1.Now()
		bdpl.Status.LastReconcile = &now
		err = r.client.Status().Update(ctx, bdpl)
		if err != nil {
			return reconcile.Result{},
				log.WithEvent(bdpl, "UpdateError").Errorf(ctx, "failed to update reconcile timestamp on bdpl '%s' (%v): %s", request.NamespacedName, bdpl.ResourceVersion, err)
		}
		log.Infof(ctx, "Meltdown started for '%s'", request.NamespacedName)

		return reconcile.Result{RequeueAfter: ReconcileSkipDuration}, nil
	}

	if meltdown.NewWindow(ReconcileSkipDuration, bdpl.Status.LastReconcile).Contains(time.Now()) {
		log.Infof(ctx, "Meltdown in progress for '%s'", request.NamespacedName)
		return reconcile.Result{}, nil
	}

	log.Infof(ctx, "Meltdown ended for '%s'", request.NamespacedName)

	bdpl.Status.LastReconcile = nil
	// update the bdpl state spec with the initial state
	now := metav1.Now()
	bdpl.Status.StateTimestamp = &now
	bdpl.Status.State = BDPLStateCreating

	err = r.client.Status().Update(ctx, bdpl)
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(bdpl, "UpdateError").Errorf(ctx, "failed to update reconcile timestamp on bdpl '%s' (%v): %s", request.NamespacedName, bdpl.ResourceVersion, err)
	}

	manifest, err := r.resolveManifest(ctx, bdpl)
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(bdpl, "WithOpsManifestError").Errorf(ctx, "failed to get with-ops manifest for BOSHDeployment '%s': %v", request.NamespacedName, err)
	}

	// Find the required native-to-bosh links, add the properties to the manifest and error if links are missing
	l := linkInfoService{
		log:            logger.TraceFilter(log.ExtractLogger(ctx), "linkinfoservice"),
		deploymentName: bdpl.Name,
		namespace:      bdpl.Namespace,
	}
	linkInfos, err := l.List(ctx, r.client, manifest)
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(bdpl, "InstanceGroupManifestError").Errorf(ctx, "failed to find native quarks-links for BOSHDeployment '%s': %v", request.NamespacedName, err)
	}

	// delete qsts which are not in the manifest
	err = r.deleteQuarksStatefulSets(ctx, manifest, bdpl)
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(bdpl, "DeleteQuarksStatefulSet").Error(ctx, "failed to delete orphan QuarksStatefulSets", err)
	}

	// Create all QuarksSecret variables
	log.Debug(ctx, "Converting BOSH manifest variables to QuarksSecret resources")
	secrets, err := r.converter.Variables(request.Namespace, bdpl.Name, manifest.Variables)
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(bdpl, "BadManifestError").Error(ctx, errors.Wrap(err, "failed to generate quarks secrets from manifest"))

	}

	// Create/update all explicit BOSH Variables
	if len(secrets) > 0 {
		err = r.createQuarksSecrets(ctx, bdpl, secrets)
		if err != nil {
			return reconcile.Result{},
				log.WithEvent(bdpl, "VariableGenerationError").Errorf(ctx, "failed to create quarks secrets for BOSH manifest '%s': %v", request.NamespacedName, err)
		}
	}

	// Apply the "Instance group manifest" QuarksJob, which creates instance group manifests (ig-resolved) secrets and BPM config secrets
	// once the "Variable Interpolation" job created the desired manifest.
	qJob, err := r.jobFactory.InstanceGroupManifestJob(request.Namespace, bdpl.Name, *manifest, linkInfos, bdpl.ObjectMeta.Generation == 1)
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(bdpl, "InstanceGroupManifestError").Errorf(ctx, "failed to build instance group manifest qJob: %v", err)
	}

	log.Debug(ctx, "Creating instance group manifest QuarksJob")
	err = r.createQuarksJob(ctx, bdpl, qJob)
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(bdpl, "InstanceGroupManifestError").Errorf(ctx, "failed to create instance group manifest qJob for BOSHDeployment '%s': %v", request.NamespacedName, err)
	}

	// Apply the "with-ops" manifest secret
	log.Debug(ctx, "Creating with-ops manifest secret")
	err = r.createManifestWithOps(ctx, bdpl, *manifest)
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(bdpl, "WithOpsManifestError").Errorf(ctx, "failed to create with-ops manifest secret for BOSHDeployment '%s': %v", request.NamespacedName, err)
	}

	return reconcile.Result{}, nil
}

// resolveManifest resolves manifest with ops manifest
func (r *ReconcileBOSHDeployment) resolveManifest(ctx context.Context, bdpl *bdv1.BOSHDeployment) (*bdm.Manifest, error) {
	log.Debug(ctx, "Resolving manifest")
	manifest, _, err := r.withops.Manifest(ctx, bdpl, bdpl.GetNamespace())
	if err != nil {
		return nil, log.WithEvent(bdpl, "WithOpsManifestError").Errorf(ctx, "Error resolving the manifest '%s': %s", bdpl.GetNamespacedName(), err)
	}

	return manifest, nil
}

// createManifestWithOps creates a secret containing the deployment manifest with ops files applied
func (r *ReconcileBOSHDeployment) createManifestWithOps(ctx context.Context, bdpl *bdv1.BOSHDeployment, manifest bdm.Manifest) error {
	log.Debug(ctx, "Creating manifest secret with ops")

	// Create manifest with ops, which will be used as a base for variable interpolation in desired manifest job input.
	manifestBytes, err := manifest.Marshal()
	if err != nil {
		return log.WithEvent(bdpl, "ManifestWithOpsMarshalError").Errorf(ctx, "Error marshaling the manifest '%s': %s", bdpl.GetNamespacedName(), err)
	}

	manifestSecretName := bdv1.DeploymentSecretTypeManifestWithOps.String()

	// Create a secret object for the manifest
	manifestSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      manifestSecretName,
			Namespace: bdpl.GetNamespace(),
			Labels: map[string]string{
				bdv1.LabelDeploymentName:       bdpl.Name,
				bdv1.LabelDeploymentSecretType: bdv1.DeploymentSecretTypeManifestWithOps.String(),
			},
		},
		StringData: map[string]string{
			"manifest.yaml": string(manifestBytes),
		},
	}

	// Set ownership reference
	if err := r.setReference(bdpl, manifestSecret, r.scheme); err != nil {
		return log.WithEvent(bdpl, "ManifestWithOpsRefError").Errorf(ctx, "failed to set ownerReference for Secret '%s/%s': %v", bdpl.Namespace, manifestSecretName, err)
	}

	// Apply the secret
	op, err := controllerutil.CreateOrUpdate(ctx, r.client, manifestSecret, mutateqs.SecretMutateFn(manifestSecret))
	if err != nil {
		return log.WithEvent(bdpl, "ManifestWithOpsApplyError").Errorf(ctx, "failed to apply Secret '%s/%s': %v", bdpl.Namespace, manifestSecretName, err)
	}

	log.Debugf(ctx, "ResourceReference secret '%s/%s' has been %s", bdpl.Namespace, manifestSecret.Name, op)

	return nil
}

// createQuarksJob creates a QuarksJob and sets its ownership
func (r *ReconcileBOSHDeployment) createQuarksJob(ctx context.Context, bdpl *bdv1.BOSHDeployment, qJob *qjv1a1.QuarksJob) error {
	if err := r.setReference(bdpl, qJob, r.scheme); err != nil {
		return errors.Errorf("failed to set ownerReference for QuarksJob '%s/%s': %v", bdpl.Namespace, qJob.GetName(), err)
	}

	op, err := controllerutil.CreateOrUpdate(ctx, r.client, qJob, mutate.QuarksJobMutateFn(qJob))
	if err != nil {
		return errors.Wrapf(err, "creating or updating QuarksJob '%s/%s'", bdpl.Namespace, qJob.Name)
	}

	log.Debugf(ctx, "QuarksJob '%s/%s' has been %s", bdpl.Namespace, qJob.Name, op)

	return err
}

// createQuarksSecrets create variables quarksSecrets
func (r *ReconcileBOSHDeployment) createQuarksSecrets(ctx context.Context, bdpl *bdv1.BOSHDeployment, variables []qsv1a1.QuarksSecret) error {

	// TODO: vladi: don't generate the variables that are "user-defined"

	for _, variable := range variables {
		log.Debugf(ctx, "CreateOrUpdate QuarksSecrets for explicit variable '%s'", variable.GetNamespacedName())

		// Set the "manifest with ops" secret as the owner for the QuarksSecrets
		// The "manifest with ops" secret is owned by the actual BOSHDeployment, so everything
		// should be garbage collected properly.
		if err := r.setReference(bdpl, &variable, r.scheme); err != nil {
			return log.WithEvent(bdpl, "OwnershipError").Errorf(ctx, "failed to set ownership for '%s': %v", variable.GetNamespacedName(), err)
		}

		op, err := controllerutil.CreateOrUpdate(ctx, r.client, &variable, mutateqs.QuarksSecretMutateFn(&variable))
		if err != nil {
			return errors.Wrapf(err, "creating or updating QuarksSecret '%s'", variable.GetNamespacedName())
		}

		// Update does not update status. We only trigger quarks secret
		// reconciler again if variable was updated by previous CreateOrUpdate
		if op == controllerutil.OperationResultUpdated {
			variable.Status.Copied = pointers.Bool(false)
			variable.Status.Generated = pointers.Bool(false)
			if err := r.client.Status().Update(ctx, &variable); err != nil {
				return log.WithEvent(&variable, "UpdateError").Errorf(ctx, "failed to update generated status on quarks secret '%s' (%v): %s", variable.GetNamespacedName(), variable.ResourceVersion, err)
			}
		}

		log.Debugf(ctx, "QuarksSecret '%s' has been %s", variable.GetNamespacedName(), op)
	}

	return nil
}

// deleteQuarksStatefulSets deletes qsts which are removed from the manifest
func (r *ReconcileBOSHDeployment) deleteQuarksStatefulSets(ctx context.Context, manifest *bdm.Manifest, bdpl *bdv1.BOSHDeployment) error {
	quarksStatefulSets := &qstsv1a1.QuarksStatefulSetList{}
	err := r.client.List(ctx, quarksStatefulSets, client.InNamespace(bdpl.Namespace))
	if err != nil {
		return errors.Wrap(err, "failed to list QuarksStatefulSets")
	}

	qstsToBeDeleted := []qstsv1a1.QuarksStatefulSet{}
	for _, qsts := range quarksStatefulSets.Items {
		labels := qsts.GetLabels()
		if labels[bdv1.LabelDeploymentName] == bdpl.Name {
			_, found := manifest.InstanceGroups.InstanceGroupByName(labels[bdv1.LabelInstanceGroupName])
			if !found {
				qstsToBeDeleted = append(qstsToBeDeleted, qsts)
			}
		}
	}

	for _, qsts := range qstsToBeDeleted {
		log.Infof(ctx, "deleting quarksstatefulset '%s'", qsts.Name)
		err = r.client.Delete(ctx, &qsts)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}

		// delete all associated services
		services := &corev1.ServiceList{}
		name := qsts.Labels[bdv1.LabelInstanceGroupName]
		labels := map[string]string{bdv1.LabelInstanceGroupName: name}
		err := r.client.List(ctx, services, client.InNamespace(bdpl.Namespace), client.MatchingLabels(labels))
		if err != nil {
			return errors.Wrapf(err, "failed to list services for instance group %s", name)
		}
		for _, svc := range services.Items {
			err = r.client.Delete(ctx, &svc)
			if err != nil {
				if apierrors.IsNotFound(err) {
					return nil
				}
				return err
			}
		}
	}

	return nil
}
