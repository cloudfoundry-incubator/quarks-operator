package extendedjob

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"strings"

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
	podName := request.NamespacedName.Name

	pod := &corev1.Pod{}
	err = r.client.Get(context.TODO(), request.NamespacedName, pod)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// do not requeue, pod is probably deleted
			r.log.Debugf("Failed to find pod, not retrying: %s", err)
			err = nil
			return
		}
		// Error reading the object - requeue the request.
		r.log.Errorf("Failed to get the pod: %s", err)
		return
	}

	podState := InferPodState(*pod)
	if podState == ejv1.PodStateUnknown {
		r.log.Debugf(
			"Failed to determine state %s: %#v",
			PodStatusString(*pod),
			pod.Status,
		)
		return
	}

	extJobs := &ejv1.ExtendedJobList{}
	err = r.client.List(context.TODO(), &client.ListOptions{}, extJobs)
	if err != nil {
		r.log.Infof("Failed to query extended jobs: %s", err)
		return
	}

	if len(extJobs.Items) < 1 {
		// maybe we should requeue, so this is not lost for future jobs?
		r.log.Debugf("no extendedjobs found")
		return
	}

	podEvent := fmt.Sprintf("%s/%s", podName, podState)
	r.log.Debugf("Considering %d extended jobs for pod %s", len(extJobs.Items), podEvent)

	for _, extJob := range extJobs.Items {
		if r.query.MatchState(extJob, podState) && r.query.Match(extJob, *pod) {
			err := r.createJob(extJob, podName)
			if err != nil {
				if apierrors.IsAlreadyExists(err) {
					r.log.Debugf("Skip '%s' triggered by pod %s: already running", extJob.Name, podEvent)
				} else {
					r.log.Infof("Failed to create job for '%s' via pod %s: %s", extJob.Name, podEvent, err)
				}
				continue
			}
			r.log.Infof("Created job for '%s' via pod %s", extJob.Name, podEvent)
		}
	}

	return
}

func (r *TriggerReconciler) createJob(extJob ejv1.ExtendedJob, podName string) error {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName(extJob.Name, podName),
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
		r.log.Errorf("Failed to set owner reference on job for '%s' via pod %s: %s", extJob.Name, podName, err)
	}

	err = r.client.Update(context.TODO(), job)
	if err != nil {
		r.log.Errorf("Failed to update job with owner reference for '%s': %s", extJob.Name, err)
		return err
	}

	return nil
}

// jobName returns a unique, short name for a given extJob, pod combination
// k8s allows 63 chars, but the pod will have -\d{6} appended
// IDEA: maybe use pod.Uid instead of rand
func jobName(extJobName, podName string) string {
	hashID := randSuffix(fmt.Sprintf("%s-%s", extJobName, podName))
	return fmt.Sprintf("job-%s-%s-%s", truncate(extJobName, 15), truncate(podName, 15), hashID)
}

func randSuffix(str string) string {
	randBytes := make([]byte, 16)
	rand.Read(randBytes)
	a := fnv.New64()
	a.Write([]byte(str + string(randBytes)))
	return hex.EncodeToString(a.Sum(nil))
}

func truncate(name string, max int) string {
	name = strings.Replace(name, "-", "", -1)
	if len(name) > max {
		return name[0:max]
	}
	return name
}
