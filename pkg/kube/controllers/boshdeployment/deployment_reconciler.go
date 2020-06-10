package boshdeployment

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	qjv1a1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/quarksjob/v1alpha1"
	"code.cloudfoundry.org/quarks-operator/pkg/bosh/converter"
	bdm "code.cloudfoundry.org/quarks-operator/pkg/bosh/manifest"
	bdv1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/boshdns"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/mutate"
	qsv1a1 "code.cloudfoundry.org/quarks-secret/pkg/kube/apis/quarkssecret/v1alpha1"
	mutateqs "code.cloudfoundry.org/quarks-secret/pkg/kube/util/mutate"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	log "code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	"code.cloudfoundry.org/quarks-utils/pkg/meltdown"
	"code.cloudfoundry.org/quarks-utils/pkg/pointers"
)

// JobFactory creates Jobs for a given manifest
type JobFactory interface {
	VariableInterpolationJob(namespace string, deploymentName string, manifest bdm.Manifest) (*qjv1a1.QuarksJob, error)
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

	if meltdown.NewWindow(r.config.MeltdownDuration, bdpl.Status.LastReconcile).Contains(time.Now()) {
		log.WithEvent(bdpl, "Meltdown").Debugf(ctx, "Resource '%s' is in meltdown, requeue reconcile after %s", request.NamespacedName, r.config.MeltdownRequeueAfter)
		return reconcile.Result{RequeueAfter: r.config.MeltdownRequeueAfter}, nil
	}

	manifest, err := r.resolveManifest(ctx, bdpl)
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(bdpl, "WithOpsManifestError").Errorf(ctx, "failed to get with-ops manifest for BOSHDeployment '%s': %v", request.NamespacedName, err)
	}

	// Get link infos containing provider name and its secret name
	linkInfos, err := r.listLinkInfos(bdpl, manifest)
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(bdpl, "InstanceGroupManifestError").Errorf(ctx, "failed to list quarks-link secrets for BOSHDeployment '%s': %v", request.NamespacedName, err)
	}

	// Apply the "with-ops" manifest secret
	log.Debug(ctx, "Creating with-ops manifest secret")
	manifestSecret, err := r.createManifestWithOps(ctx, bdpl, *manifest)
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(bdpl, "WithOpsManifestError").Errorf(ctx, "failed to create with-ops manifest secret for BOSHDeployment '%s': %v", request.NamespacedName, err)
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
		err = r.createQuarksSecrets(ctx, manifestSecret, secrets)
		if err != nil {
			return reconcile.Result{},
				log.WithEvent(bdpl, "VariableGenerationError").Errorf(ctx, "failed to create quarks secrets for BOSH manifest '%s': %v", request.NamespacedName, err)
		}
	}

	// Apply the "Variable Interpolation" QuarksJob, which creates the desired manifest secret
	qJob, err := r.jobFactory.VariableInterpolationJob(request.Namespace, bdpl.Name, *manifest)
	if err != nil {
		return reconcile.Result{}, log.WithEvent(bdpl, "DesiredManifestError").Errorf(ctx, "failed to build the desired manifest qJob: %v", err)
	}

	log.Debug(ctx, "Creating desired manifest QuarksJob")
	err = r.createQuarksJob(ctx, bdpl, qJob)
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(bdpl, "DesiredManifestError").Errorf(ctx, "failed to create desired manifest qJob for BOSHDeployment '%s': %v", request.NamespacedName, err)
	}

	// Apply the "Instance group manifest" QuarksJob, which creates instance group manifests (ig-resolved) secrets and BPM config secrets
	// once the "Variable Interpolation" job created the desired manifest.
	qJob, err = r.jobFactory.InstanceGroupManifestJob(request.Namespace, bdpl.Name, *manifest, linkInfos, bdpl.ObjectMeta.Generation == 1)
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

	// Update status of bdpl with the timestamp of the last reconcile
	now := metav1.Now()
	bdpl.Status.LastReconcile = &now

	err = r.client.Status().Update(ctx, bdpl)
	if err != nil {
		log.WithEvent(bdpl, "UpdateError").Errorf(ctx, "failed to update reconcile timestamp on bdpl '%s' (%v): %s", request.NamespacedName, bdpl.ResourceVersion, err)
		return reconcile.Result{Requeue: false}, nil
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
func (r *ReconcileBOSHDeployment) createManifestWithOps(ctx context.Context, bdpl *bdv1.BOSHDeployment, manifest bdm.Manifest) (*corev1.Secret, error) {
	log.Debug(ctx, "Creating manifest secret with ops")

	// Create manifest with ops, which will be used as a base for variable interpolation in desired manifest job input.
	manifestBytes, err := manifest.Marshal()
	if err != nil {
		return nil, log.WithEvent(bdpl, "ManifestWithOpsMarshalError").Errorf(ctx, "Error marshaling the manifest '%s': %s", bdpl.GetNamespacedName(), err)
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
		return nil, log.WithEvent(bdpl, "ManifestWithOpsRefError").Errorf(ctx, "failed to set ownerReference for Secret '%s/%s': %v", bdpl.Namespace, manifestSecretName, err)
	}

	// Apply the secret
	op, err := controllerutil.CreateOrUpdate(ctx, r.client, manifestSecret, mutateqs.SecretMutateFn(manifestSecret))
	if err != nil {
		return nil, log.WithEvent(bdpl, "ManifestWithOpsApplyError").Errorf(ctx, "failed to apply Secret '%s/%s': %v", bdpl.Namespace, manifestSecretName, err)
	}

	log.Debugf(ctx, "ResourceReference secret '%s/%s' has been %s", bdpl.Namespace, manifestSecret.Name, op)

	return manifestSecret, nil
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

// listLinkInfos returns a LinkInfos containing link providers if needed
// and updates `quarks_links` properties
func (r *ReconcileBOSHDeployment) listLinkInfos(bdpl *bdv1.BOSHDeployment, manifest *bdm.Manifest) (converter.LinkInfos, error) {
	linkInfos := converter.LinkInfos{}

	// find all missing providers in the manifest, so we can look for secrets
	missingProviders := manifest.ListMissingProviders()

	// quarksLinks store for missing provider names with types read from secrets
	quarksLinks := map[string]bdm.QuarksLink{}
	if len(missingProviders) != 0 {
		// list secrets and services from target deployment
		secrets := &corev1.SecretList{}
		err := r.client.List(r.ctx, secrets,
			client.InNamespace(bdpl.Namespace),
		)
		if err != nil {
			return linkInfos, errors.Wrapf(err, "listing secrets for link in deployment '%s':", bdpl.GetNamespacedName())
		}

		services := &corev1.ServiceList{}
		err = r.client.List(r.ctx, services,
			client.InNamespace(bdpl.Namespace),
		)
		if err != nil {
			return linkInfos, errors.Wrapf(err, "listing services for link in deployment '%s':", bdpl.GetNamespacedName())
		}

		for _, s := range secrets.Items {
			if deploymentName, ok := s.GetAnnotations()[bdv1.LabelDeploymentName]; ok && deploymentName == bdpl.Name {
				linkProvider, err := newLinkProvider(s.GetAnnotations())
				if err != nil {
					return linkInfos, errors.Wrapf(err, "failed to parse link JSON for '%s'", bdpl.GetNamespacedName())
				}
				if dup, ok := missingProviders[linkProvider.Name]; ok {
					if dup {
						return linkInfos, errors.New(fmt.Sprintf("duplicated secrets of provider: %s", linkProvider.Name))
					}

					linkInfos = append(linkInfos, converter.LinkInfo{
						SecretName:   s.Name,
						ProviderName: linkProvider.Name,
						ProviderType: linkProvider.ProviderType,
					})

					if linkProvider.ProviderType != "" {
						quarksLinks[linkProvider.Name] = bdm.QuarksLink{
							Type: linkProvider.ProviderType,
						}
					}
					missingProviders[linkProvider.Name] = true
				}
			}
		}

		serviceRecords, err := r.getServiceRecords(bdpl.Namespace, bdpl.Name, services.Items)
		if err != nil {
			return linkInfos, errors.Wrapf(err, "failed to get link services for '%s'", bdpl.GetNamespacedName())
		}

		// Update quarksLinks section `manifest.Properties["quarks_links"]` with info from existing serviceRecords
		for qName := range quarksLinks {
			if svcRecord, ok := serviceRecords[qName]; ok {

				var jobsInstances []bdm.JobInstance

				if svcRecord.selector != nil {
					// Service has selectors, we're going through pods in order to build
					// an instance list for the link
					pods, err := r.listPodsFromSelector(bdpl.Namespace, svcRecord.selector)
					if err != nil {
						return linkInfos, errors.Wrapf(err, "Failed to get link pods for '%s'", bdpl.GetNamespacedName())
					}

					for i, p := range pods {
						if len(p.Status.PodIP) == 0 {
							return linkInfos, fmt.Errorf("empty ip of kube native component: '%s'", p.Name)
						}
						jobsInstances = append(jobsInstances, bdm.JobInstance{
							Name:      qName,
							ID:        string(p.GetUID()),
							Index:     i,
							Address:   p.Status.PodIP,
							Bootstrap: i == 0,
						})
					}
				} else if svcRecord.addresses != nil {
					for i, a := range svcRecord.addresses {
						jobsInstances = append(jobsInstances, bdm.JobInstance{
							Name:      qName,
							ID:        a,
							Index:     i,
							Address:   a,
							Bootstrap: i == 0,
						})
					}
				} else {
					// No selector, no addresses - we're creating one instance that just points to the service address itself
					jobsInstances = append(jobsInstances, bdm.JobInstance{
						Name:      qName,
						ID:        qName,
						Index:     0,
						Address:   svcRecord.dnsRecord,
						Bootstrap: true,
					})
				}

				quarksLinks[qName] = bdm.QuarksLink{
					Type:      quarksLinks[qName].Type,
					Address:   svcRecord.dnsRecord,
					Instances: jobsInstances,
				}
			}
		}
	}

	missingPs := make([]string, 0, len(missingProviders))
	for key, found := range missingProviders {
		if !found {
			missingPs = append(missingPs, key)
		}
	}

	if len(missingPs) != 0 {
		return linkInfos, errors.New(fmt.Sprintf("missing link secrets for providers: %s", strings.Join(missingPs, ", ")))
	}

	if len(quarksLinks) != 0 {
		if manifest.Properties == nil {
			manifest.Properties = map[string]interface{}{}
		}
		manifest.Properties[bdm.QuarksLinksProperty] = quarksLinks
	}

	return linkInfos, nil
}

// getServiceRecords gets service records from Kube Services
func (r *ReconcileBOSHDeployment) getServiceRecords(namespace string, name string, svcs []corev1.Service) (map[string]serviceRecord, error) {
	svcRecords := map[string]serviceRecord{}
	for _, svc := range svcs {
		if deploymentName, ok := svc.GetAnnotations()[bdv1.LabelDeploymentName]; ok && deploymentName == name {
			if providerName, ok := svc.GetAnnotations()[bdv1.AnnotationLinkProviderService]; ok {
				if _, ok := svcRecords[providerName]; ok {
					return svcRecords, errors.New(fmt.Sprintf("duplicated services of provider: %s", providerName))
				}

				// An ExternalName service doesn't have a selector or endpoints
				if svc.Spec.Type == corev1.ServiceTypeExternalName {
					svcRecords[providerName] = serviceRecord{
						addresses: nil,
						selector:  nil,
						dnsRecord: fmt.Sprintf("%s.%s.svc.%s", svc.Name, namespace, boshdns.GetClusterDomain()),
					}

					continue
				}

				if len(svc.Spec.Selector) != 0 {
					svcRecords[providerName] = serviceRecord{
						addresses: nil,
						selector:  svc.Spec.Selector,
						dnsRecord: fmt.Sprintf("%s.%s.svc.%s", svc.Name, namespace, boshdns.GetClusterDomain()),
					}

					continue
				}

				// If we don't have a selector, we're either dealing with an ExternalName service,
				// or a service that's backed by manually created Endpoints.
				endpoints := &corev1.Endpoints{}
				err := r.client.Get(
					r.ctx,
					types.NamespacedName{
						Name:      svc.Name,
						Namespace: svc.Namespace,
					},
					endpoints)

				if err != nil {
					// No selectors and no endpoints
					if apierrors.IsNotFound(err) {
						svcRecords[providerName] = serviceRecord{
							addresses: nil,
							selector:  nil,
							dnsRecord: fmt.Sprintf("%s.%s.svc.%s", svc.Name, namespace, boshdns.GetClusterDomain()),
						}
					}

					// We hit an actual error
					return nil, errors.Wrapf(err, "failed to get service endpoints for links")
				}

				addresses := []string{}
				for _, subset := range endpoints.Subsets {
					for _, address := range subset.Addresses {
						addresses = append(addresses, address.IP)
					}
				}

				svcRecords[providerName] = serviceRecord{
					addresses: addresses,
					selector:  nil,
					dnsRecord: fmt.Sprintf("%s.%s.svc.%s", svc.Name, namespace, boshdns.GetClusterDomain()),
				}
			}
		}
	}

	return svcRecords, nil
}

// listPodsFromSelector lists pods from the selector
func (r *ReconcileBOSHDeployment) listPodsFromSelector(namespace string, selector map[string]string) ([]corev1.Pod, error) {
	podList := &corev1.PodList{}
	err := r.client.List(r.ctx, podList,
		client.InNamespace(namespace),
		client.MatchingLabels(selector),
	)
	if err != nil {
		return podList.Items, errors.Wrapf(err, "listing pods from selector '%+v':", selector)
	}

	return podList.Items, nil
}

// createQuarksSecrets create variables quarksSecrets
func (r *ReconcileBOSHDeployment) createQuarksSecrets(ctx context.Context, manifestSecret *corev1.Secret, variables []qsv1a1.QuarksSecret) error {

	// TODO: vladi: don't generate the variables that are "user-defined"

	for _, variable := range variables {
		log.Debugf(ctx, "CreateOrUpdate QuarksSecrets for explicit variable '%s'", variable.GetNamespacedName())

		// Set the "manifest with ops" secret as the owner for the QuarksSecrets
		// The "manifest with ops" secret is owned by the actual BOSHDeployment, so everything
		// should be garbage collected properly.
		if err := r.setReference(manifestSecret, &variable, r.scheme); err != nil {
			return log.WithEvent(manifestSecret, "OwnershipError").Errorf(ctx, "failed to set ownership for '%s': %v", variable.GetNamespacedName(), err)
		}

		op, err := controllerutil.CreateOrUpdate(ctx, r.client, &variable, mutateqs.QuarksSecretMutateFn(&variable))
		if err != nil {
			return errors.Wrapf(err, "creating or updating QuarksSecret '%s'", variable.GetNamespacedName())
		}

		// Update does not update status. We only trigger quarks secret
		// reconciler again if variable was updated by previous CreateOrUpdate
		if op == controllerutil.OperationResultUpdated {
			variable.Status.Generated = pointers.Bool(false)
			if err := r.client.Status().Update(ctx, &variable); err != nil {
				return log.WithEvent(&variable, "UpdateError").Errorf(ctx, "failed to update generated status on quarks secret '%s' (%v): %s", variable.GetNamespacedName(), variable.ResourceVersion, err)
			}
		}

		log.Debugf(ctx, "QuarksSecret '%s' has been %s", variable.GetNamespacedName(), op)
	}

	return nil
}

type serviceRecord struct {
	selector  map[string]string
	dnsRecord string
	addresses []string
}
