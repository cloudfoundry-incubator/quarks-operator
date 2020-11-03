package quarksdrain

import (
	"context"
	"errors"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"code.cloudfoundry.org/quarks-utils/pkg/config"
	log "code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
)

// NewDrainReconciler returns a new reconciler to add finalizers for pods with drain scripts
func NewDrainReconciler(ctx context.Context, config *config.Config, mgr manager.Manager) reconcile.Reconciler {
	return &DrainReconciler{
		ctx:    ctx,
		config: config,
		client: mgr.GetClient(),
	}
}

// DrainReconciler contains necessary state for the reconcile
type DrainReconciler struct {
	ctx    context.Context
	client client.Client
	config *config.Config
}

// Helper functions to check and remove string from a slice of strings.
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}

// Reconcile adds a finalizer to the pods with the pre-drain annotation
func (r *DrainReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	pod := &corev1.Pod{}

	// Set the ctx to be Background, as the top-level context for incoming requests.
	ctx, cancel := context.WithTimeout(r.ctx, r.config.CtxTimeOut)
	defer cancel()

	finalizerName := "quarks.cloudfoundry.org/finalizer"
	// examine DeletionTimestamp to determine if object is under deletion
	if pod.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !containsString(pod.ObjectMeta.Finalizers, finalizerName) {
			pod.ObjectMeta.Finalizers = append(pod.ObjectMeta.Finalizers, finalizerName)
			if err := r.client.Update(ctx, pod); err != nil {
				return reconcile.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if containsString(pod.ObjectMeta.Finalizers, finalizerName) {
			// our finalizer is present, so lets handle any external dependency
			if r.drainRunning(pod) {
				log.WithEvent(pod, "DrainRunning").Infof(ctx, "container still in execution '%s/%s' (%v): %s", pod.Namespace, pod.Name, pod.ResourceVersion)
				// if fail to delete the external dependency here, return with error
				// so that it can be retried
				return reconcile.Result{}, errors.New("Draining scripts still running")
			}

			// remove our finalizer from the list and update it.
			pod.ObjectMeta.Finalizers = removeString(pod.ObjectMeta.Finalizers, finalizerName)
			if err := r.client.Update(ctx, pod); err != nil {
				return reconcile.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return reconcile.Result{}, nil
	}

	log.Info(ctx, "Reconciling pod ", request.NamespacedName)
	err := r.client.Get(ctx, request.NamespacedName, pod)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Return and don't requeue
			log.Debug(ctx, "Skip pod reconcile: pod not found")
			return reconcile.Result{}, nil
		}

		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// if meltdown.NewAnnotationWindow(r.config.MeltdownDuration, pod.ObjectMeta.Annotations).Contains(time.Now()) {
	// 	log.WithEvent(pod, "Meltdown").Debugf(ctx, "Resource '%s/%s' is in meltdown, requeue reconcile after %s", pod.Namespace, pod.Name, r.config.MeltdownRequeueAfter)
	// 	return reconcile.Result{RequeueAfter: r.config.MeltdownRequeueAfter}, nil
	// }

	// meltdown.SetLastReconcile(&pod.ObjectMeta, time.Now())
	// err = r.client.Update(ctx, pod)
	// if err != nil {
	// 	log.WithEvent(pod, "UpdateError").Errorf(ctx, "Failed to update reconcile timestamp on restart annotated pod '%s/%s' (%v): %s", pod.Namespace, pod.Name, pod.ResourceVersion, err)
	// 	return reconcile.Result{}, nil
	// }
	return reconcile.Result{}, nil
}

func (r *DrainReconciler) drainRunning(pod *corev1.Pod) bool {
	// Check if drain script is done
	// drain script: container.Lifecycle.PreStop
	// https://github.com/cloudfoundry-incubator/quarks-operator/blob/16c2610f5650437279d95463651dee44dfa8828e/pkg/bosh/bpmconverter/container_factory.go#L539
	// drain is running?
	annotations := pod.GetAnnotations()
	containerName, found := annotations[PreDrain]
	if !found {
		return false
	}
	for i := range pod.Status.ContainerStatuses {
		c := &pod.Status.ContainerStatuses[i]
		// e.g. garden-garden
		if c.Name == containerName {
			return c.State.Terminated != nil
		}
	}

	return true
}
