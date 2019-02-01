package extendedjob

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
)

var _ reconcile.Reconciler = &TriggerReconciler{}

type setOwnerReferenceFunc func(owner, object metav1.Object, scheme *runtime.Scheme) error

// NewTriggerReconciler returns a new reconcile to start jobs triggered by pods
func NewTriggerReconciler(
	log *zap.SugaredLogger,
	mgr manager.Manager,
	query Query,
	f setOwnerReferenceFunc,
) reconcile.Reconciler {
	return &TriggerReconciler{
		client:            mgr.GetClient(),
		log:               log,
		query:             query,
		recorder:          mgr.GetRecorder("extendedjob trigger reconciler"),
		scheme:            mgr.GetScheme(),
		setOwnerReference: f,
	}
}

// TriggerReconciler implements the Reconciler interface
type TriggerReconciler struct {
	client            client.Client
	log               *zap.SugaredLogger
	query             Query
	recorder          record.EventRecorder
	scheme            *runtime.Scheme
	setOwnerReference setOwnerReferenceFunc
}

// Reconcile creates jobs for extended jobs which match the request's pod.
// When there are multiple extendedjobs, multiple jobs can run for the same
// pod.
func (r *TriggerReconciler) Reconcile(request reconcile.Request) (result reconcile.Result, err error) {

	r.log.Infof("TriggerReconcer considering extended jobs for pod %s", request.NamespacedName)

	pod := &corev1.Pod{}
	err = r.client.Get(context.TODO(), request.NamespacedName, pod)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			// Error reading the object - requeue the request.
			r.log.Errorf("failed to get the pod: %s", err)
			return reconcile.Result{}, err
		}
	}

	extJobs := &ejv1.ExtendedJobList{}
	err = r.client.List(context.TODO(), &client.ListOptions{}, extJobs)
	if err != nil {
		r.log.Infof("failed to query extended jobs: %s", err)
		return
	}

	if len(extJobs.Items) < 1 {
		return
	}

	for _, extJob := range extJobs.Items {
		if r.query.Match(extJob, *pod) {
			r.log.Infof("%s: triggered for pod %s", extJob.Name, pod.Name)

			err := r.createJob(extJob, *pod)
			if err != nil {
				// Job names are unique, so AlreadyExists can happen.
				if apierrors.IsAlreadyExists(err) {
					r.log.Debugf("%s: skipped job for pod %s: already running", extJob.Name, pod.Name)
				} else {
					r.log.Infof("%s: failed to create job for pod %s: %s", extJob.Name, pod.Name, err)
				}
				continue
			}
			r.log.Infof("%s: created job for pod %s", extJob.Name, pod.Name)

		}
	}

	return
}

func (r *TriggerReconciler) createJob(extJob ejv1.ExtendedJob, pod corev1.Pod) error {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("job-%s-%s", extJob.Name, pod.Name),
			Namespace: extJob.Namespace,
			Labels:    map[string]string{"extendedjob": "true"},
		},
		Spec: batchv1.JobSpec{Template: extJob.Spec.Template},
	}

	err := r.client.Create(context.TODO(), job)
	if err != nil {
		return err
	}

	err = r.setOwnerReference(&extJob, job, r.scheme)
	if err != nil {
		r.log.Errorf("%s: failed to set reference on job for pod %s: %s", extJob.Name, pod.Name, err)
	}

	return nil
}
