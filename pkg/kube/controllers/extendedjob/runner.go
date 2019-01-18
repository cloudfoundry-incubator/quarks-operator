package extendedjob

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"go.uber.org/zap"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
)

var _ Runner = &RunnerImpl{}

type setReferenceFunc func(owner, object metav1.Object, scheme *runtime.Scheme) error

// Runner starts jobs for extended job definitions
type Runner interface {
	Run()
}

// NewRunner returns a new runner struct
func NewRunner(
	log *zap.SugaredLogger,
	mgr manager.Manager,
	query Query,
	srf setReferenceFunc,
) *RunnerImpl {
	return &RunnerImpl{
		client:       mgr.GetClient(),
		log:          log,
		query:        query,
		recorder:     mgr.GetRecorder("extendedjob runner"),
		scheme:       mgr.GetScheme(),
		setReference: srf,
	}
}

// RunnerImpl implements the Runner interface
type RunnerImpl struct {
	client       client.Client
	log          *zap.SugaredLogger
	query        Query
	recorder     record.EventRecorder
	scheme       *runtime.Scheme
	setReference setReferenceFunc
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

	var podEvents []PodEvent
	podEvents, err = r.query.FetchPods(events)
	if err != nil {
		r.log.Infof("failed to get pods for events: %s", err)
		return
	}

	for _, extJob := range extJobs.Items {
		tsName := jobTimestampName(extJob.Name)
		filtered := r.query.Match(extJob, podEvents)
		// if multiple events belong to the same pod `query.Match` will return
		// all of the (event, pod) tuples.
		// However we update the timestamp in the first iteration of
		// this loop. Further iterations won't run the same job twice
		// on the same pod, since the updated timestamp will be newer
		// than any event we look at in this loop.
		for _, podEvent := range filtered {
			if r.isFreshStamp(tsName, podEvent) {
				continue
			}

			job, err := r.createJob(extJob, podEvent.Pod)
			// Job names are unique, so AlreadyExists happens a lot
			// for long running jobs, between multiple invocations
			// of Run().
			if err != nil && !errors.IsAlreadyExists(err) {
				r.log.Infof("failed to create job for %s: %s", extJob.Name, err)
				continue
			}

			err = r.setReference(&extJob, job, r.scheme)
			if err != nil {
				r.log.Infof("failed to set reference on job for %s: %s", extJob.Name, err)
			}

			err = r.updateStamp(tsName, podEvent)
			if err != nil {
				r.log.Infof("failed to update job stamp on pod %s: %s", podEvent.Pod.Name, err)
			}
		}
	}
}

func (r *RunnerImpl) createJob(extJob v1alpha1.ExtendedJob, pod corev1.Pod) (*batchv1.Job, error) {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("job-%s-%s", extJob.Name, pod.Name),
			Namespace: extJob.Namespace,
		},
		Spec: jobSpec(),
	}

	err := r.client.Create(context.TODO(), job)
	return job, err
}

// return true if job timestamp on pod is newer than event timestamp
func (r *RunnerImpl) isFreshStamp(tsName string, podEvent PodEvent) bool {
	annotations := podEvent.Pod.Annotations
	if ts, ok := annotations[tsName]; ok {

		n, err := strconv.ParseInt(ts, 10, 64)
		if err != nil {
			r.log.Debugf("cannot parse job timestamp annotation on pod %s", podEvent.Pod.Name)
			return false
		}

		podStamp := time.Unix(n, 0)
		eventStamp := podEvent.Event.LastTimestamp.Time
		return eventStamp.Before(podStamp)
	}
	r.log.Debugf("no job annotation on pod, ok for first time: %s", podEvent.Pod.Name)
	return false
}

func (r *RunnerImpl) updateStamp(tsName string, podEvent PodEvent) error {
	ts := podEvent.Event.LastTimestamp.Time.Unix()
	annotations := podEvent.Pod.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	annotations[tsName] = strconv.FormatInt(ts, 10)
	podEvent.Pod.SetAnnotations(annotations)
	return r.client.Update(context.TODO(), &podEvent.Pod)
}

func jobTimestampName(name string) string {
	return fmt.Sprintf("job-ts-%s", name)
}

func jobSpec() batchv1.JobSpec {
	one := int64(1)
	return batchv1.JobSpec{
		Template: corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				RestartPolicy:                 corev1.RestartPolicyNever,
				TerminationGracePeriodSeconds: &one,
				Containers: []corev1.Container{
					{
						Name:    "busybox",
						Image:   "busybox",
						Command: []string{"sleep", "6"},
					},
				},
			},
		},
	}

}
