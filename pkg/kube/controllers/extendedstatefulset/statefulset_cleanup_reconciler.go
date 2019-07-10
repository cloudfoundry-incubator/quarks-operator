package extendedstatefulset

import (
	"context"
	"strconv"

	"github.com/pkg/errors"
	"k8s.io/api/apps/v1beta2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	estsv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
	podutil "code.cloudfoundry.org/cf-operator/pkg/kube/util/pod"
)

// NewStatefulSetCleanupReconciler returns a new reconcile.Reconciler
func NewStatefulSetCleanupReconciler(ctx context.Context, config *config.Config, mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileStatefulSetCleanup{
		ctx:    ctx,
		config: config,
		client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
	}
}

// ReconcileStatefulSetCleanup reconciles an ExtendedStatefulSet object when references changes
type ReconcileStatefulSetCleanup struct {
	ctx    context.Context
	client client.Client
	scheme *runtime.Scheme
	config *config.Config
}

// Reconcile cleans up old versions and volumeManagement statefulSet of the ExtendedStatefulSet
func (r *ReconcileStatefulSetCleanup) Reconcile(request reconcile.Request) (reconcile.Result, error) {

	// Fetch the ExtendedStatefulSet we need to reconcile
	exStatefulSet := &estsv1.ExtendedStatefulSet{}

	// Set the ctx to be Background, as the top-level context for incoming requests.
	ctx, cancel := context.WithTimeout(r.ctx, r.config.CtxTimeOut)
	defer cancel()

	ctxlog.Info(ctx, "Reconciling ExtendedStatefulSet ", request.NamespacedName)
	err := r.client.Get(ctx, request.NamespacedName, exStatefulSet)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			ctxlog.Debug(ctx, "Skip reconcile: ExtendedStatefulSet not found")
			return reconcile.Result{}, nil
		}

		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Cleanup volumeManagement statefulSet once it's all pods are ready
	err = r.deleteVolumeManagementStatefulSet(ctx, exStatefulSet)
	if err != nil {
		ctxlog.Error(ctx, "Could not delete volumeManagement statefulSet of ExtendedStatefulSet '", request.NamespacedName, "': ", err)
		return reconcile.Result{}, err
	}

	statefulSetVersions, err := r.listStatefulSetVersions(ctx, exStatefulSet)
	if err != nil {
		return reconcile.Result{}, err
	}

	maxAvailableVersion := exStatefulSet.GetMaxAvailableVersion(statefulSetVersions)

	// Clean up versions when there is more than one version
	if len(statefulSetVersions) > 1 {
		err = r.cleanupStatefulSets(ctx, exStatefulSet, maxAvailableVersion)
		if err != nil {
			return reconcile.Result{}, ctxlog.WithEvent(exStatefulSet, "CleanupError").Error(ctx, "Could not cleanup StatefulSets owned by ExtendedStatefulSet '", request.NamespacedName, "': ", err)
		}
	}

	return reconcile.Result{}, nil
}

// listStatefulSetVersions gets all StatefulSets' versions and ready status owned by the ExtendedStatefulSet
func (r *ReconcileStatefulSetCleanup) listStatefulSetVersions(ctx context.Context, exStatefulSet *estsv1.ExtendedStatefulSet) (map[int]bool, error) {
	ctxlog.Debug(ctx, "Listing StatefulSets owned by ExtendedStatefulSet '", exStatefulSet.Name, "'.")

	versions := map[int]bool{}

	statefulSets, err := listStatefulSets(ctx, r.client, exStatefulSet)
	if err != nil {
		return nil, err
	}

	for _, statefulSet := range statefulSets {
		strVersion, found := statefulSet.Annotations[estsv1.AnnotationVersion]
		if !found {
			return versions, errors.Errorf("version annotation is not found from: %+v", statefulSet.Annotations)
		}

		version, err := strconv.Atoi(strVersion)
		if err != nil {
			return versions, errors.Wrapf(err, "version annotation is not an int: %s", strVersion)
		}

		ready, err := r.isStatefulSetReady(ctx, &statefulSet)
		if err != nil {
			return nil, err
		}

		versions[version] = ready
	}

	return versions, nil
}

// cleanupStatefulSets cleans up StatefulSets and versions if they are no longer required
func (r *ReconcileStatefulSetCleanup) cleanupStatefulSets(ctx context.Context, exStatefulSet *estsv1.ExtendedStatefulSet, maxAvailableVersion int) error {
	ctxlog.WithEvent(exStatefulSet, "CleanupStatefulSets").Infof(ctx, "Cleaning up StatefulSets for ExtendedStatefulSet '%s' less than version %d.", exStatefulSet.Name, maxAvailableVersion)

	statefulSets, err := listStatefulSets(ctx, r.client, exStatefulSet)
	if err != nil {
		return errors.Wrapf(err, "couldn't list StatefulSets for cleanup")
	}

	for _, statefulSet := range statefulSets {
		ctxlog.Debugf(ctx, "Considering StatefulSet '%s' for cleanup", statefulSet.Name)

		strVersion, found := statefulSet.Annotations[estsv1.AnnotationVersion]
		if !found {
			return errors.Errorf("version annotation is not found from: %+v", statefulSet.Annotations)
		}

		version, err := strconv.Atoi(strVersion)
		if err != nil {
			return errors.Wrapf(err, "version annotation is not an int: %s", strVersion)
		}

		if version >= maxAvailableVersion {
			continue
		}

		ctxlog.Debugf(ctx, "Deleting StatefulSet '%s'", statefulSet.Name)
		err = r.client.Delete(ctx, &statefulSet, client.PropagationPolicy(metav1.DeletePropagationBackground))
		if err != nil {
			return ctxlog.WithEvent(exStatefulSet, "DeleteError").Errorf(ctx, "Could not delete StatefulSet '%s': %v", statefulSet.Name, err)
		}
	}

	return nil
}

// isStatefulSetReady returns true if at least one pod owned by the StatefulSet is running
func (r *ReconcileStatefulSetCleanup) isStatefulSetReady(ctx context.Context, statefulSet *v1beta2.StatefulSet) (bool, error) {
	labelsSelector := labels.Set{
		v1beta2.StatefulSetRevisionLabel: statefulSet.Status.CurrentRevision,
	}

	podList := &corev1.PodList{}
	err := r.client.List(
		ctx,
		&client.ListOptions{
			Namespace:     statefulSet.Namespace,
			LabelSelector: labelsSelector.AsSelector(),
		},
		podList,
	)
	if err != nil {
		return false, err
	}

	for _, pod := range podList.Items {
		if metav1.IsControlledBy(&pod, statefulSet) {
			if podutil.IsPodReady(&pod) {
				ctxlog.Debugf(ctx, "Pod '%s' owned by StatefulSet '%s' is running.", pod.Name, statefulSet.Name)
				return true, nil
			}
		}
	}

	return false, nil
}
