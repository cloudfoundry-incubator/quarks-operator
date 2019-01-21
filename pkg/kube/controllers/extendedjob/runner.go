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

	"code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
)

var _ Runner = &RunnerImpl{}

type setOwnerReferenceFunc func(owner, object metav1.Object, scheme *runtime.Scheme) error

// Runner starts jobs for extended job definitions
type Runner interface {
	Run()
}

// NewRunner returns a new runner struct
func NewRunner(
	log *zap.SugaredLogger,
	mgr manager.Manager,
	query Query,
	f setOwnerReferenceFunc,
) *RunnerImpl {
	return &RunnerImpl{
		client:            mgr.GetClient(),
		log:               log,
		query:             query,
		recorder:          mgr.GetRecorder("extendedjob runner"),
		scheme:            mgr.GetScheme(),
		setOwnerReference: f,
	}
}

// RunnerImpl implements the Runner interface
type RunnerImpl struct {
	client            client.Client
	log               *zap.SugaredLogger
	query             Query
	recorder          record.EventRecorder
	scheme            *runtime.Scheme
	setOwnerReference setOwnerReferenceFunc
}

// Run jobs for all pods involved in recent events
// One event involves one pod, an extendedjob might match the pod's labels.
// We compare timestamps to decide if the extendjob has already been run for that pod.
// If the timestamp is older, we finally start the job.
// This is done for every valid combination of (pod, extendedjob) filtered from events.
// When there are multiple extendedjobs, multiple jobs can run for the same pod.
// When an extendedjob matches several pods, it will start the job for each pod.
func (r *RunnerImpl) Run() {
	extJobs := &v1alpha1.ExtendedJobList{}
	err := r.client.List(context.TODO(), &client.ListOptions{}, extJobs)
	if err != nil {
		r.log.Infof("failed to query extended jobs: %s", err)
		return
	}

	if len(extJobs.Items) < 1 {
		return
	}

	events, err := r.query.RecentPodEvents()
	if err != nil {
		r.log.Infof("failed to query pod related events: %s", err)
		return
	}

	// we need to load all pods, otherwise we can't look at labels
	var podEvents []PodEvent
	podEvents, err = r.query.FetchPods(events)
	if err != nil {
		r.log.Infof("failed to get pods for events: %s", err)
		return
	}

	for _, extJob := range extJobs.Items {
		filtered := r.query.Match(extJob, podEvents)
		// if multiple events belong to the same pod `query.Match` will return
		// all of the (event, pod) tuples.
		// However we update the timestamp in the first iteration of
		// this loop. Further iterations won't run the same job twice
		// on the same pod, since the updated timestamp will be newer
		// than any event we look at in this loop.
		for _, podEvent := range filtered {
			// this might be too noisy
			r.log.Debugf("%s: looking at pod event: %s/%s for pod %s", extJob.Name, podEvent.Event.Name, podEvent.Event.Reason, podEvent.podName())

			if podEvent.isOld(extJob.Name) {
				continue
			}

			err := r.createJob(extJob, *podEvent.Pod)
			if err != nil {
				// Job names are unique, so AlreadyExists happens a lot
				// for long running jobs, between multiple invocations
				// of Run().
				if apierrors.IsAlreadyExists(err) {
					r.log.Debugf("%s: skipped job for pod %s: already running", extJob.Name, podEvent.podName())
				} else {
					r.log.Infof("%s: failed to create job for pod %s: %s", extJob.Name, podEvent.podName(), err)
				}
				continue
			}
			r.log.Infof("%s: created job for pod %s", extJob.Name, podEvent.podName())

			err = podEvent.setTimestamp(r.client, extJob.Name)
			if err != nil {
				r.log.Infof("%s: failed to update job timestamp on pod %s: %s", extJob.Name, podEvent.podName(), err)
			}
		}
	}
}

func (r *RunnerImpl) createJob(extJob v1alpha1.ExtendedJob, pod corev1.Pod) error {
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
