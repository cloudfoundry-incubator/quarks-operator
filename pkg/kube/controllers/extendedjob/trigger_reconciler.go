package extendedjob

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"strings"

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
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
)

var _ reconcile.Reconciler = &TriggerReconciler{}

type setOwnerReferenceFunc func(owner, object metav1.Object, scheme *runtime.Scheme) error

// NewTriggerReconciler returns a new reconcile to start jobs triggered by pods
func NewTriggerReconciler(
	ctx context.Context,
	config *config.Config,
	mgr manager.Manager,
	query Query,
	f setOwnerReferenceFunc,
) reconcile.Reconciler {
	return &TriggerReconciler{
		ctx:               ctx,
		client:            mgr.GetClient(),
		config:            config,
		query:             query,
		recorder:          mgr.GetRecorder("extendedjob trigger reconciler"),
		scheme:            mgr.GetScheme(),
		setOwnerReference: f,
	}
}

// TriggerReconciler implements the Reconciler interface
type TriggerReconciler struct {
	ctx               context.Context
	client            client.Client
	config            *config.Config
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
		ctxlog.Debugf(ctx,
			"Failed to determine state %s: %#v",
			PodStatusString(*pod),
			pod.Status,
		)
		return
	}

	extJobs := &ejv1.ExtendedJobList{}
	err = r.client.List(ctx, &client.ListOptions{}, extJobs)
	if err != nil {
		ctxlog.Infof(ctx, "Failed to query extended jobs: %s", err)
		return
	}

	if len(extJobs.Items) < 1 {
		return
	}

	podEvent := fmt.Sprintf("%s/%s", podName, podState)
	ctxlog.Debugf(ctx, "Considering %d extended jobs for pod %s", len(extJobs.Items), podEvent)

	for _, extJob := range extJobs.Items {
		if r.query.MatchState(extJob, podState) && r.query.Match(extJob, *pod) {
			err := r.createJob(ctx, extJob, podName)
			if err != nil {
				if apierrors.IsAlreadyExists(err) {
					ctxlog.Debugf(ctx, "Skip '%s' triggered by pod %s: already running", extJob.Name, podEvent)
				} else {
					ctxlog.Infof(ctx, "Failed to create job for '%s' via pod %s: %s", extJob.Name, podEvent, err)
				}
				continue
			}
			ctxlog.Infof(ctx, "Created job for '%s' via pod %s", extJob.Name, podEvent)
		}
	}
	return
}

func (r *TriggerReconciler) createJob(ctx context.Context, extJob ejv1.ExtendedJob, podName string) error {
	template := extJob.Spec.Template.DeepCopy()

	if template.Labels == nil {
		template.Labels = map[string]string{}
	}
	template.Labels["ejob-name"] = extJob.Name

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName(extJob.Name, podName),
			Namespace: extJob.Namespace,
			Labels:    map[string]string{"extendedjob": "true"},
		},
		Spec: batchv1.JobSpec{Template: *template},
	}

	err := r.setOwnerReference(&extJob, job, r.scheme)
	if err != nil {
		ctxlog.Errorf(ctx, "Failed to set owner reference on job for '%s' via pod %s: %s", extJob.Name, podName, err)
	}

	err = r.client.Create(ctx, job)
	if err != nil {
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
