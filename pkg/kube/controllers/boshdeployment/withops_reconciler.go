package boshdeployment

import (
	"context"
	"encoding/json"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	bdm "code.cloudfoundry.org/quarks-operator/pkg/bosh/manifest"
	bdv1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/boshdns"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	log "code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	"code.cloudfoundry.org/quarks-utils/pkg/meltdown"
	"code.cloudfoundry.org/quarks-utils/pkg/versionedsecretstore"
)

// InterpolateSecrets renders manifest variables from quarkssecrets.
type InterpolateSecrets interface {
	InterpolateVariableFromSecrets(ctx context.Context, withOpsManifestData []byte, namespace string, boshdeploymentName string) ([]byte, error)
}

// NewDNSFunc returns a dns client for the manifest
type NewDNSFunc func(m bdm.Manifest) (boshdns.DomainNameService, error)

// NewWithOpsReconciler returns a new reconcile.Reconciler
func NewWithOpsReconciler(ctx context.Context, config *config.Config, mgr manager.Manager, resolver InterpolateSecrets, srf setReferenceFunc, dns NewDNSFunc) reconcile.Reconciler {
	return &ReconcileWithOps{
		ctx:                  ctx,
		config:               config,
		client:               mgr.GetClient(),
		scheme:               mgr.GetScheme(),
		resolver:             resolver,
		setReference:         srf,
		versionedSecretStore: versionedsecretstore.NewVersionedSecretStore(mgr.GetClient()),
		newDNSFunc:           dns,
	}
}

// ReconcileWithOps reconciles the with ops manifest secret
type ReconcileWithOps struct {
	ctx                  context.Context
	config               *config.Config
	client               client.Client
	scheme               *runtime.Scheme
	resolver             InterpolateSecrets
	setReference         setReferenceFunc
	versionedSecretStore versionedsecretstore.VersionedSecretStore
	newDNSFunc           NewDNSFunc
}

// Reconcile reconciles an withOps secret and generates the corresponding
// desired manifest secret.
func (r *ReconcileWithOps) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Set the ctx to be Background, as the top-level context for incoming requests.
	ctx, cancel := context.WithTimeout(r.ctx, r.config.CtxTimeOut)
	defer cancel()

	log.Infof(ctx, "Reconciling withOps secret '%s'", request.NamespacedName)
	withOpsSecret := &corev1.Secret{}
	err := r.client.Get(ctx, request.NamespacedName, withOpsSecret)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Debug(ctx, "Skip reconcile: WithOps secret not found")
			return reconcile.Result{}, nil
		}

		// Error reading the object - requeue the request.
		err = log.WithEvent(withOpsSecret, "GetWithOpsSecret").Errorf(ctx, "Failed to get withOps secret '%s': %v", request.NamespacedName, err)
		if err != nil {
			return reconcile.Result{RequeueAfter: time.Second * 5}, err
		}
		return reconcile.Result{RequeueAfter: time.Second * 5}, nil
	}

	annotations := withOpsSecret.GetAnnotations()
	if len(annotations) == 0 {
		annotations = map[string]string{}
	}
	labels := withOpsSecret.GetLabels()
	boshdeploymentName := labels[bdv1.LabelDeploymentName]

	lastReconcile, ok := annotations[meltdown.AnnotationLastReconcile]
	if (ok && lastReconcile == "") || !ok {
		annotations[meltdown.AnnotationLastReconcile] = metav1.Now().Format(time.RFC3339)
		withOpsSecret.SetAnnotations(annotations)
		err = r.client.Update(ctx, withOpsSecret)
		if err != nil {
			return reconcile.Result{},
				log.WithEvent(withOpsSecret, "UpdateError").Errorf(ctx, "failed to update lastreconcile annotation on withops secret for bdpl '%s': %v", boshdeploymentName, err)
		}
		log.Infof(ctx, "Meltdown started for '%s'", request.NamespacedName)

		return reconcile.Result{RequeueAfter: ReconcileSkipDuration}, nil
	}

	if meltdown.NewAnnotationWindow(ReconcileSkipDuration, annotations).Contains(time.Now()) {
		log.Infof(ctx, "Meltdown in progress for '%s'", request.NamespacedName)
		return reconcile.Result{}, nil
	}
	log.Infof(ctx, "Meltdown ended for '%s'", request.NamespacedName)

	boshdeployment := &bdv1.BOSHDeployment{}
	err = r.client.Get(ctx, types.NamespacedName{Namespace: request.Namespace, Name: boshdeploymentName}, boshdeployment)
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(withOpsSecret, "WithOpsManifestError").Errorf(ctx, "failed to get BOSHDeployment '%s': %v", boshdeploymentName, err)
	}

	annotations[meltdown.AnnotationLastReconcile] = ""
	withOpsSecret.SetAnnotations(annotations)
	err = r.client.Update(ctx, withOpsSecret)
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(withOpsSecret, "UpdateError").Errorf(ctx, "failed to update lastreconcile annotation on withops secret for bdpl '%s': %v", boshdeploymentName, err)
	}

	withOpsManifestData := withOpsSecret.Data["manifest.yaml"]

	log.Infof(ctx, "Interpolating variables")
	desiredManifestBytes, err := r.resolver.InterpolateVariableFromSecrets(ctx, withOpsManifestData, request.Namespace, boshdeploymentName)
	if err != nil {
		return reconcile.Result{RequeueAfter: time.Second * 5},
			log.WithEvent(withOpsSecret, "WithOpsManifestError").Errorf(ctx, "failed to interpolated variables for BOSHDeployment '%s': %v", boshdeploymentName, err)
	}

	log.Infof(ctx, "Creating desired manifest secret")
	err = r.createDesiredManifest(ctx, desiredManifestBytes, *boshdeployment, request.Namespace)
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(withOpsSecret, "WithOpsManifestError").Errorf(ctx, "failed to create desired manifest secret for BOSHDeployment '%s': %v", boshdeploymentName, err)
	}

	manifest, err := bdm.LoadYAML(desiredManifestBytes)
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(withOpsSecret, "WithOpsManifestError").Errorf(ctx, "failed to unmarshal manifest bytes for boshdeployment '%s': %v", boshdeploymentName, err)
	}

	dns, err := r.newDNSFunc(*manifest)
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(withOpsSecret, "WithOpsManifestError").Errorf(ctx, "failed to create desired manifest secret for BOSHDeployment '%s': %v", boshdeploymentName, err)
	}

	err = dns.Apply(ctx, request.Namespace, r.client, func(object metav1.Object) error {
		return r.setReference(boshdeployment, object, r.scheme)
	})
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(withOpsSecret, "WithOpsManifestError").Errorf(ctx, "Failed to reconcile dns: %v", err)
	}

	return reconcile.Result{}, nil
}

// createDesiredManifest creates a secret containing the deployment manifest with ops files applied and variables interpolated
func (r *ReconcileWithOps) createDesiredManifest(ctx context.Context, desiredManifestBytes []byte, boshdeployment bdv1.BOSHDeployment, namespace string) error {

	desiredManifestJSONBytes, err := json.Marshal(map[string]string{
		bdm.DesiredManifestKeyName: string(desiredManifestBytes),
	})
	if err != nil {
		return err
	}

	var desiredManifestData map[string]string
	if err := json.Unmarshal([]byte(desiredManifestJSONBytes), &desiredManifestData); err != nil {
		return err
	}

	desiredManifestSecretName := "desired-manifest"
	secretLabels := map[string]string{
		bdv1.LabelDeploymentName:       boshdeployment.Name,
		bdv1.LabelDeploymentSecretType: bdv1.DeploymentSecretTypeDesiredManifest.String(),
	}
	secretAnnotations := map[string]string{}
	sourceDescription := "created by quarksOperator"

	store := versionedsecretstore.NewVersionedSecretStore(r.client)
	err = store.Create(context.Background(), namespace, boshdeployment.Name,
		boshdeployment.GetUID(), boshdeployment.Kind, desiredManifestSecretName, desiredManifestData,
		secretAnnotations, secretLabels, sourceDescription)
	if err != nil {
		if !versionedsecretstore.IsSecretIdenticalError(err) {
			return err
		}
		// No-op. the latest version is identical to the one we have
		return nil
	}
	log.Infof(ctx, "Secret '%s/%s' has been created", namespace, desiredManifestSecretName)

	return nil
}
