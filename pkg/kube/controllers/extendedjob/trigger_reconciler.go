package extendedjob

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
	podutil "code.cloudfoundry.org/cf-operator/pkg/kube/util/pod"
	vss "code.cloudfoundry.org/cf-operator/pkg/kube/util/versionedsecretstore"
)

var _ reconcile.Reconciler = &TriggerReconciler{}

// NewTriggerReconciler returns a new reconcile to start jobs triggered by pods
func NewTriggerReconciler(
	ctx context.Context,
	config *config.Config,
	mgr manager.Manager,
	query Query,
	f setOwnerReferenceFunc,
	store vss.VersionedSecretStore,
) reconcile.Reconciler {
	jc := NewJobCreator(mgr.GetClient(), mgr.GetScheme(), f, store)

	return &TriggerReconciler{
		ctx:               ctx,
		client:            mgr.GetClient(),
		config:            config,
		query:             query,
		scheme:            mgr.GetScheme(),
		setOwnerReference: f,
		jobCreator:        jc,
	}
}

// TriggerReconciler implements the Reconciler interface
type TriggerReconciler struct {
	ctx               context.Context
	client            client.Client
	config            *config.Config
	query             Query
	scheme            *runtime.Scheme
	setOwnerReference setOwnerReferenceFunc
	jobCreator        JobCreator
}

// Reconcile creates jobs for extended jobs which match the request's pod.
// When there are multiple extendedjobs, multiple jobs can run for the same
// pod.
func (r *TriggerReconciler) Reconcile(request reconcile.Request) (result reconcile.Result, err error) {
	podName := request.NamespacedName.Name

	pod := &corev1.Pod{}

	// Set the ctx to be Background, as the top-level context for incoming requests.
	ctx, cancel := context.WithTimeout(r.ctx, r.config.CtxTimeOut)
	defer cancel()

	err = r.client.Get(ctx, request.NamespacedName, pod)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// do not requeue, pod is probably deleted
			ctxlog.Debugf(ctx, "Failed to find pod, not retrying: %s", err)
			err = nil
			return
		}
		// Error reading the object - requeue the request.
		ctxlog.Errorf(ctx, "Failed to get the pod: %s", err)
		return
	}

	podState := InferPodState(*pod)
	if podState == ejv1.PodStateUnknown {
		ctxlog.Debugf(ctx, "Failed to determine state %s", podutil.GetPodStatusString(*pod))
		return
	}

	eJobs := &ejv1.ExtendedJobList{}
	err = r.client.List(ctx, eJobs)
	if err != nil {
		ctxlog.Infof(ctx, "Failed to query extended jobs: %s", err)
		return
	}

	if len(eJobs.Items) < 1 {
		return
	}

	podEvent := fmt.Sprintf("%s/%s", podName, podState)
	ctxlog.Debugf(ctx, "Considering %d extended jobs for pod %s", len(eJobs.Items), podEvent)

	for _, eJob := range eJobs.Items {
		if r.query.MatchState(eJob, podState) && r.query.Match(eJob, *pod) {
			if _, err := r.jobCreator.Create(ctx, eJob, podName, string(pod.UID)); err != nil {
				ctxlog.WithEvent(&eJob, "CreateJob").Infof(ctx, "Failed to create job for '%s' via pod %s: %s", eJob.Name, podEvent, err)
				continue
			}

			ctxlog.WithEvent(&eJob, "CreateJob").Infof(ctx, "Created job for '%s' via pod %s", eJob.Name, podEvent)
		}
	}
	return
}
