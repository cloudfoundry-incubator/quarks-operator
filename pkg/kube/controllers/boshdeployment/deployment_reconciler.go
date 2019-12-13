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
	"sigs.k8s.io/controller-runtime/pkg/client"
	crc "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/converter"
	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/mutate"
	qjv1a1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/quarksjob/v1alpha1"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	log "code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	"code.cloudfoundry.org/quarks-utils/pkg/meltdown"
	"code.cloudfoundry.org/quarks-utils/pkg/names"
)

// JobFactory creates Jobs for a given manifest
type JobFactory interface {
	VariableInterpolationJob(manifest bdm.Manifest) (*qjv1a1.QuarksJob, error)
	InstanceGroupManifestJob(manifest bdm.Manifest, linkInfos converter.LinkInfos, initialRollout bool) (*qjv1a1.QuarksJob, error)
	BPMConfigsJob(manifest bdm.Manifest, linkInfos converter.LinkInfos, initialRollout bool) (*qjv1a1.QuarksJob, error)
}

// Check that ReconcileBOSHDeployment implements the reconcile.Reconciler interface
var _ reconcile.Reconciler = &ReconcileBOSHDeployment{}

type setReferenceFunc func(owner, object metav1.Object, scheme *runtime.Scheme) error

// NewDeploymentReconciler returns a new reconcile.Reconciler
func NewDeploymentReconciler(ctx context.Context, config *config.Config, mgr manager.Manager, resolver converter.Resolver, jobFactory JobFactory, srf setReferenceFunc) reconcile.Reconciler {

	return &ReconcileBOSHDeployment{
		ctx:          ctx,
		config:       config,
		client:       mgr.GetClient(),
		scheme:       mgr.GetScheme(),
		resolver:     resolver,
		setReference: srf,
		jobFactory:   jobFactory,
	}
}

// ReconcileBOSHDeployment reconciles a BOSHDeployment object
type ReconcileBOSHDeployment struct {
	ctx          context.Context
	config       *config.Config
	client       client.Client
	scheme       *runtime.Scheme
	resolver     converter.Resolver
	setReference setReferenceFunc
	jobFactory   JobFactory
}

// Reconcile starts the deployment process for a BOSHDeployment and deploys QuarksJobs to generate required properties for instance groups and rendered BPM
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

		return reconcile.Result{},
			log.WithEvent(instance, "GetBOSHDeploymentError").Errorf(ctx, "failed to get BOSHDeployment '%s': %v", request.NamespacedName, err)
	}

	if meltdown.NewWindow(r.config.MeltdownDuration, instance.Status.LastReconcile).Contains(time.Now()) {
		log.WithEvent(instance, "Meltdown").Debugf(ctx, "Resource '%s' is in meltdown, requeue reconcile after %s", instance.Name, r.config.MeltdownRequeueAfter)
		return reconcile.Result{RequeueAfter: r.config.MeltdownRequeueAfter}, nil
	}

	// Resolve the manifest with ops
	manifest, err := r.resolveManifest(ctx, instance)
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(instance, "WithOpsManifestError").Errorf(ctx, "failed to get with-ops manifest for BOSHDeployment '%s': %v", request.NamespacedName, err)
	}

	// Get link infos containing provider name and its secret name
	linkInfos, err := r.listLinkInfos(instance, manifest)
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(instance, "InstanceGroupManifestError").Errorf(ctx, "failed to list quarks-link secrets for BOSHDeployment '%s': %v", request.NamespacedName, err)
	}

	// Apply the "with-ops" manifest secret
	log.Debug(ctx, "Creating with-ops manifest secret")
	err = r.createManifestWithOps(ctx, instance, *manifest)
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(instance, "WithOpsManifestError").Errorf(ctx, "failed to create with-ops manifest secret for BOSHDeployment '%s': %v", request.NamespacedName, err)
	}

	log.Debug(ctx, "Rendering manifest")

	// Apply the "Variable Interpolation" QuarksJob, which creates the desired manifest secret
	qJob, err := r.jobFactory.VariableInterpolationJob(*manifest)
	if err != nil {
		return reconcile.Result{}, log.WithEvent(instance, "DesiredManifestError").Errorf(ctx, "failed to build the desired manifest qJob: %v", err)
	}

	log.Debug(ctx, "Creating desired manifest QuarksJob")
	err = r.createQuarksJob(ctx, instance, qJob)
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(instance, "DesiredManifestError").Errorf(ctx, "failed to create desired manifest qJob for BOSHDeployment '%s': %v", request.NamespacedName, err)
	}

	// Apply the "Instance group manifest" QuarksJob, which creates instance group manifests (ig-resolved) secrets
	qJob, err = r.jobFactory.InstanceGroupManifestJob(*manifest, linkInfos, instance.ObjectMeta.Generation == 1)
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(instance, "InstanceGroupManifestError").Errorf(ctx, "failed to build instance group manifest qJob: %v", err)
	}

	log.Debug(ctx, "Creating instance group manifest QuarksJob")
	err = r.createQuarksJob(ctx, instance, qJob)
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(instance, "InstanceGroupManifestError").Errorf(ctx, "failed to create instance group manifest qJob for BOSHDeployment '%s': %v", request.NamespacedName, err)
	}

	// Apply the "BPM Configs" QuarksJob, which creates BPM config secrets
	qJob, err = r.jobFactory.BPMConfigsJob(*manifest, linkInfos, instance.ObjectMeta.Generation == 1)
	if err != nil {
		return reconcile.Result{}, log.WithEvent(instance, "BPMConfigsError").Errorf(ctx, "failed to build BPM configs qJob: %v", err)

	}
	log.Debug(ctx, "Creating BPM configs QuarksJob")
	err = r.createQuarksJob(ctx, instance, qJob)
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(instance, "BPMConfigsError").Errorf(ctx, "failed to create BPM configs qJob for BOSHDeployment '%s': %v", request.NamespacedName, err)
	}

	// Update status of bdpl with the timestamp of the last reconcile
	now := metav1.Now()
	instance.Status.LastReconcile = &now

	err = r.client.Status().Update(ctx, instance)
	if err != nil {
		log.WithEvent(instance, "UpdateError").Errorf(ctx, "failed to update reconcile timestamp on bdpl '%s' (%v): %s", instance.Name, instance.ResourceVersion, err)
		return reconcile.Result{Requeue: false}, nil
	}

	return reconcile.Result{}, nil
}

// resolveManifest resolves manifest with ops manifest
func (r *ReconcileBOSHDeployment) resolveManifest(ctx context.Context, instance *bdv1.BOSHDeployment) (*bdm.Manifest, error) {
	log.Debug(ctx, "Resolving manifest")
	manifest, _, err := r.resolver.WithOpsManifest(ctx, instance, instance.GetNamespace())
	if err != nil {
		return nil, log.WithEvent(instance, "WithOpsManifestError").Errorf(ctx, "Error resolving the manifest %s: %s", instance.GetName(), err)
	}

	// Replace the name with the name of the BOSHDeployment resource
	manifest.Name = instance.GetName()

	return manifest, nil
}

// createManifestWithOps creates a secret containing the deployment manifest with ops files applied
func (r *ReconcileBOSHDeployment) createManifestWithOps(ctx context.Context, instance *bdv1.BOSHDeployment, manifest bdm.Manifest) error {
	log.Debug(ctx, "Creating manifest secret with ops")

	// Create manifest with ops, which will be used as a base for variable interpolation in desired manifest job input.
	manifestBytes, err := manifest.Marshal()
	if err != nil {
		return log.WithEvent(instance, "ManifestWithOpsMarshalError").Errorf(ctx, "Error marshaling the manifest %s: %s", instance.GetName(), err)
	}

	manifestSecretName := names.DeploymentSecretName(names.DeploymentSecretTypeManifestWithOps, manifest.Name, "")

	// Create a secret object for the manifest
	manifestSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      manifestSecretName,
			Namespace: instance.GetNamespace(),
		},
		StringData: map[string]string{
			"manifest.yaml": string(manifestBytes),
		},
	}

	// Set ownership reference
	if err := r.setReference(instance, manifestSecret, r.scheme); err != nil {
		return log.WithEvent(instance, "ManifestWithOpsRefError").Errorf(ctx, "failed to set ownerReference for Secret '%s': %v", manifestSecretName, err)
	}

	// Apply the secret
	op, err := controllerutil.CreateOrUpdate(ctx, r.client, manifestSecret, mutate.SecretMutateFn(manifestSecret))
	if err != nil {
		return log.WithEvent(instance, "ManifestWithOpsApplyError").Errorf(ctx, "failed to apply Secret '%s': %v", manifestSecretName, err)
	}

	log.Debugf(ctx, "ResourceReference secret '%s' has been %s", manifestSecret.Name, op)

	return nil
}

// createQuarksJob creates a QuarksJob and sets its ownership
func (r *ReconcileBOSHDeployment) createQuarksJob(ctx context.Context, instance *bdv1.BOSHDeployment, qJob *qjv1a1.QuarksJob) error {
	if err := r.setReference(instance, qJob, r.scheme); err != nil {
		return errors.Errorf("failed to set ownerReference for QuarksJob '%s': %v", qJob.GetName(), err)
	}

	op, err := controllerutil.CreateOrUpdate(ctx, r.client, qJob, mutate.QuarksJobMutateFn(qJob))
	if err != nil {
		return errors.Wrapf(err, "creating or updating QuarksJob '%s'", qJob.Name)
	}

	log.Debugf(ctx, "QuarksJob '%s' has been %s", qJob.Name, op)

	return err
}

// listLinkInfos returns a LinkInfos containing link providers if needed
// and updates `quarks_links` properties
func (r *ReconcileBOSHDeployment) listLinkInfos(instance *bdv1.BOSHDeployment, manifest *bdm.Manifest) (converter.LinkInfos, error) {
	linkInfos := converter.LinkInfos{}

	// find all missing providers in the manifest, so we can look for secrets
	missingProviders := listMissingProviders(*manifest)

	// quarksLinks store for missing provider names with types read from secrets
	quarksLinks := map[string]bdm.QuarksLink{}
	if len(missingProviders) != 0 {
		// list secrets and services from target deployment
		secretLabels := map[string]string{bdv1.LabelDeploymentName: instance.Name}
		secrets := &corev1.SecretList{}
		err := r.client.List(r.ctx, secrets,
			crc.MatchingLabels(secretLabels),
			crc.InNamespace(instance.Namespace),
		)
		if err != nil {
			return linkInfos, errors.Wrapf(err, "listing secrets for link in deployment '%s':", instance.Name)
		}

		servicesLabels := map[string]string{bdv1.LabelDeploymentName: instance.Name}
		services := &corev1.ServiceList{}
		err = r.client.List(r.ctx, services,
			crc.MatchingLabels(servicesLabels),
			crc.InNamespace(instance.Namespace),
		)
		if err != nil {
			return linkInfos, errors.Wrapf(err, "listing services for link in deployment '%s':", instance.Name)
		}

		for _, s := range secrets.Items {
			providerName, ok := s.GetAnnotations()[bdv1.AnnotationLinkProviderName]
			if ok {
				if dup, ok := missingProviders[providerName]; ok {
					if dup {
						return linkInfos, errors.New(fmt.Sprintf("duplicated secrets of provider: %s", providerName))
					}

					linkInfos = append(linkInfos, converter.LinkInfo{
						SecretName:   s.Name,
						ProviderName: providerName,
					})
					if providerType, ok := s.GetAnnotations()[bdv1.AnnotationLinkProviderType]; ok {
						quarksLinks[s.Name] = bdm.QuarksLink{
							Type: providerType,
						}
					}
					missingProviders[providerName] = true
				}
			}
		}

		serviceRecords, err := r.getServiceRecords(instance.Namespace, services.Items)
		if err != nil {
			return linkInfos, errors.Wrapf(err, "failed to get link services for: %s", instance.Name)
		}

		for qName := range quarksLinks {
			if svcRecord, ok := serviceRecords[qName]; ok {
				pods, err := r.listPodsFromSelector(instance.Namespace, svcRecord.selector)
				if err != nil {
					return linkInfos, errors.Wrapf(err, "Failed to get link pods for: %s", instance.Name)
				}

				var jobsInstances []bdm.JobInstance
				for i, p := range pods {
					if len(p.Status.PodIP) == 0 {
						return linkInfos, fmt.Errorf("empty ip of kube native component: '%s/%s'", p.Namespace, p.Name)
					}
					jobsInstances = append(jobsInstances, bdm.JobInstance{
						Name:      qName,
						ID:        string(p.GetUID()),
						Index:     i,
						Address:   p.Status.PodIP,
						Bootstrap: i == 0,
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
		manifest.Properties["quarks_links"] = quarksLinks
	}

	return linkInfos, nil
}

// getServiceRecords gets service records from Kube Services
func (r *ReconcileBOSHDeployment) getServiceRecords(namespace string, svcs []corev1.Service) (map[string]serviceRecord, error) {
	svcRecords := map[string]serviceRecord{}
	for _, svc := range svcs {
		providerName, ok := svc.GetAnnotations()[bdv1.AnnotationLinkProviderName]
		if ok {
			if _, ok := svcRecords[providerName]; ok {
				return svcRecords, errors.New(fmt.Sprintf("duplicated services of provider: %s", providerName))
			}

			svcRecords[providerName] = serviceRecord{
				selector:  svc.Spec.Selector,
				dnsRecord: fmt.Sprintf("%s.%s.svc.%s", svc.Name, namespace, bdm.GetClusterDomain()),
			}
		}
	}

	return svcRecords, nil
}

// listPodsFromSelector lists pods from the selector
func (r *ReconcileBOSHDeployment) listPodsFromSelector(namespace string, selector map[string]string) ([]corev1.Pod, error) {
	podList := &corev1.PodList{}
	err := r.client.List(r.ctx, podList,
		crc.InNamespace(namespace),
		crc.MatchingLabels(selector),
	)
	if err != nil {
		return podList.Items, errors.Wrapf(err, "listing pods from selector '%+v':", selector)
	}

	if len(podList.Items) == 0 {
		return podList.Items, fmt.Errorf("got an empty list of pods")
	}

	return podList.Items, nil
}

// listMissingProviders return a list of missing providers in the manifest
func listMissingProviders(manifest bdm.Manifest) map[string]bool {
	provideAsNames := map[string]bool{}
	consumeFromNames := map[string]bool{}

	for _, ig := range manifest.InstanceGroups {
		for _, job := range ig.Jobs {
			provideAsNames = listProviderNames(job.Provides, "as")
			consumeFromNames = listProviderNames(job.Consumes, "from")
		}
	}

	// Iterate consumeFromNames and remove providers existing in the manifest
	for providerName := range consumeFromNames {
		if _, ok := provideAsNames[providerName]; ok {
			delete(consumeFromNames, providerName)
		}
	}

	return consumeFromNames
}

// listProviderNames returns a map containing provider names from job provides and consumes
func listProviderNames(providerProperties map[string]interface{}, providerKey string) map[string]bool {
	providerNames := map[string]bool{}

	for _, property := range providerProperties {
		p, ok := property.(map[string]interface{})
		if !ok {
			continue
		}
		nameVal, ok := p[providerKey]
		if !ok {
			continue
		}

		name, _ := nameVal.(string)
		if len(name) == 0 {
			continue
		}
		providerNames[name] = false
	}

	return providerNames
}

type serviceRecord struct {
	selector  map[string]string
	dnsRecord string
}
