package extendedjob

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"go.uber.org/zap"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
)

// Runner starts jobs for extended job definitions
type Runner interface {
	Run()
}

// NewRunner returns a new runner struct
func NewRunner(
	log *zap.SugaredLogger,
	mgr manager.Manager,
	query Query,
) *RunnerImpl {
	return &RunnerImpl{
		log:      log,
		client:   mgr.GetClient(),
		recorder: mgr.GetRecorder("extendedjob runner"),
		query:    query,
	}
}

// RunnerImpl implements the Runner interface
type RunnerImpl struct {
	log      *zap.SugaredLogger
	client   client.Client
	recorder record.EventRecorder
	query    Query
}

// Run jobs for all pods involved in recent events
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
		filtered := r.query.Match(extJob, podEvents)
		for _, podEvent := range filtered {
			if r.isFreshStamp(extJob, podEvent) {
				continue
			}

			// TODO set job owner
			err = r.createJob(extJob, podEvent.Pod)
			if err != nil {
				r.log.Infof("failed to create job for %s: %s", extJob.Name, err)
			}
			err = r.updateStamp(extJob, podEvent)
			if err != nil {
				r.log.Infof("failed to update job stamp on pod %s: %s", podEvent.Pod.Name, err)
			}
		}
	}
}

func (r *RunnerImpl) createJob(extJob v1alpha1.ExtendedJob, pod corev1.Pod) error {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("job-%s-%s", extJob.Name, pod.Name),
			Namespace: extJob.Namespace,
		},
		Spec: jobSpec(),
	}

	return r.client.Create(context.TODO(), job)

}

// return true if job timestamp on pod is newer than event timestamp
func (r *RunnerImpl) isFreshStamp(extJob v1alpha1.ExtendedJob, podEvent PodEvent) bool {
	annotations := podEvent.Pod.Annotations
	if ts, ok := annotations[fmt.Sprintf("job-%s", extJob.Name)]; ok {

		n, err := strconv.ParseInt(ts, 10, 64)
		if err == nil {
			return false
		}

		podStamp := time.Unix(n, 0)
		eventStamp := podEvent.Event.LastTimestamp.Time
		return eventStamp.Before(podStamp)
	}
	return false
}

func (r *RunnerImpl) updateStamp(extJob v1alpha1.ExtendedJob, podEvent PodEvent) error {
	ts := podEvent.Event.LastTimestamp.Time.Unix()
	annotations := podEvent.Pod.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	annotations[fmt.Sprintf("job-%s", extJob.Name)] = strconv.FormatInt(ts, 10)
	podEvent.Pod.SetAnnotations(annotations)
	return r.client.Update(context.TODO(), &podEvent.Pod)
}

func jobSpec() batchv1.JobSpec {
	one := int64(1)
	return batchv1.JobSpec{
		Template: corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
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
