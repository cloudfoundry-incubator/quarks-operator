package extendedjob

import (
	"code.cloudfoundry.org/cf-operator/pkg/bosh/converter/factory"
	"context"
	"encoding/json"
	"reflect"

	"github.com/pkg/errors"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/mutate"
	podutil "code.cloudfoundry.org/cf-operator/pkg/kube/util/pod"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/versionedsecretstore"
)

// NewJobReconciler returns a new Reconciler
func NewJobReconciler(ctx context.Context, config *config.Config, mgr manager.Manager, podLogGetter PodLogGetter) (reconcile.Reconciler, error) {
	versionedSecretStore := versionedsecretstore.NewVersionedSecretStore(mgr.GetClient())

	return &ReconcileJob{
		ctx:                  ctx,
		config:               config,
		client:               mgr.GetClient(),
		podLogGetter:         podLogGetter,
		scheme:               mgr.GetScheme(),
		versionedSecretStore: versionedSecretStore,
	}, nil
}

// ReconcileJob reconciles an Job object
type ReconcileJob struct {
	ctx                  context.Context
	client               client.Client
	podLogGetter         PodLogGetter
	scheme               *runtime.Scheme
	config               *config.Config
	versionedSecretStore versionedsecretstore.VersionedSecretStore
}

// Reconcile reads that state of the cluster for a Job object that is owned by an ExtendedJob and
// makes changes based on the state read and what is in the ExtendedJob.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileJob) Reconcile(request reconcile.Request) (reconcile.Result, error) {

	instance := &batchv1.Job{}

	// Set the ctx to be Background, as the top-level context for incoming requests.
	ctx, cancel := context.WithTimeout(r.ctx, r.config.CtxTimeOut)
	defer cancel()

	ctxlog.Infof(ctx, "Reconciling job output '%s' in the ExtendedJob context", request.NamespacedName)
	err := r.client.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			ctxlog.Info(ctx, "Skip reconcile: Job not found")
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		ctxlog.Info(ctx, "Error reading the object")
		return reconcile.Result{}, err
	}

	// Get the job's extended job parent
	parentName := ""
	for _, owner := range instance.GetOwnerReferences() {
		if *owner.Controller {
			parentName = owner.Name
		}
	}
	if parentName == "" {
		err = ctxlog.WithEvent(instance, "NotFoundError").Errorf(ctx, "Could not find parent ExtendedJob for Job '%s'", request.NamespacedName)
		return reconcile.Result{}, err
	}

	ej := ejv1.ExtendedJob{}
	err = r.client.Get(ctx, types.NamespacedName{Name: parentName, Namespace: instance.GetNamespace()}, &ej)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "getting parent ExtendedJob in Job Reconciler for job %s", instance.GetName())
	}

	// Persist output if needed
	if !reflect.DeepEqual(ejv1.Output{}, ej.Spec.Output) && ej.Spec.Output != nil {
		if instance.Status.Succeeded == 1 || instance.Status.Failed > *instance.Spec.BackoffLimit && ej.Spec.Output.WriteOnFailure {
			ctxlog.WithEvent(&ej, "ExtendedJob").Infof(ctx, "Persisting output of job '%s'", instance.Name)
			err = r.persistOutput(ctx, instance, ej)
			if err != nil {
				ctxlog.WithEvent(instance, "PersistOutputError").Errorf(ctx, "Could not persist output: '%s'", err)
				return reconcile.Result{Requeue: false}, nil
			}
		} else if instance.Status.Failed == 1 && !ej.Spec.Output.WriteOnFailure {
			ctxlog.WithEvent(&ej, "FailedPersistingOutput").Infof(ctx, "Will not persist output of job '%s' because it failed", instance.Name)
		} else {
			ctxlog.WithEvent(instance, "StateError").Errorf(ctx, "Job is in an unexpected state: %#v", instance)
		}
	}

	// Delete Job if it succeeded
	if instance.Status.Succeeded == 1 {
		ctxlog.WithEvent(&ej, "DeletingJob").Infof(ctx, "Deleting succeeded job '%s'", instance.Name)
		err = r.client.Delete(ctx, instance)
		if err != nil {
			ctxlog.WithEvent(instance, "DeleteError").Errorf(ctx, "Cannot delete succeeded job: '%s'", err)
		}

		if d, ok := instance.Spec.Template.Labels["delete"]; ok {
			if d == "pod" {
				pod, err := r.jobPod(ctx, instance.Name, instance.GetNamespace())
				if err != nil {
					ctxlog.WithEvent(instance, "NotFoundError").Errorf(ctx, "Cannot find job's pod: '%s'", err)
					return reconcile.Result{}, nil
				}
				ctxlog.WithEvent(&ej, "DeletingJobsPod").Infof(ctx, "Deleting succeeded job's pod '%s'", pod.Name)
				err = r.client.Delete(ctx, pod)
				if err != nil {
					ctxlog.WithEvent(instance, "DeleteError").Errorf(ctx, "Cannot delete succeeded job's pod: '%s'", err)
				}
			}
		}
	}

	return reconcile.Result{}, nil
}

// jobPod gets the job's pod. Only single-pod jobs are supported when persisting the output, so we just get the first one.
func (r *ReconcileJob) jobPod(ctx context.Context, name string, namespace string) (*corev1.Pod, error) {
	list := &corev1.PodList{}
	err := r.client.List(
		ctx,
		list,
		client.InNamespace(namespace),
		client.MatchingLabels(map[string]string{"job-name": name}),
	)
	if err != nil {
		return nil, errors.Wrapf(err, "Listing job's %s pods failed.", name)
	}
	if len(list.Items) == 0 {
		return nil, errors.Errorf("Job %s does not own any pods?", name)
	}

	// If there is only one jobpod, then return index 0 pod
	latestPod := list.Items[0]
	if len(list.Items) > 1 {
		// If there are more than one jobpods, then return the latest
		// created jobpod. There will be multiple jobpods when jobpods
		// fail. Kubernetes job creates new jobpods if the jobpod
		// created previously fails
		latestTimeStamp := list.Items[0].GetCreationTimestamp().UTC()
		for podIndex, pod := range list.Items {
			if latestTimeStamp.Before(pod.GetCreationTimestamp().UTC()) {
				latestTimeStamp = pod.GetCreationTimestamp().UTC()
				latestPod = list.Items[podIndex]
			}
		}
	}

	ctxlog.Infof(ctx, "Considering job pod %s for persisting output", latestPod.GetName())
	return &latestPod, nil
}

func (r *ReconcileJob) persistOutput(ctx context.Context, instance *batchv1.Job, ejob ejv1.ExtendedJob) error {

	pod, err := r.jobPod(ctx, instance.GetName(), instance.GetNamespace())
	if err != nil {
		return errors.Wrapf(err, "failed to persist output for ejob %s", ejob.GetName())
	}

	// Iterate over the pod's containers and store the output
	for _, c := range pod.Spec.Containers {
		result, err := r.podLogGetter.Get(instance.GetNamespace(), pod.Name, c.Name)
		if err != nil {
			return errors.Wrapf(err, "getting pod output for container %s in jobPod %s", c.Name, pod.GetName())
		}

		// Create secret
		secretName := ejob.Spec.Output.NamePrefix + c.Name

		var data map[string]string
		err = json.Unmarshal(result, &data)
		if err != nil {
			return ctxlog.WithEvent(&ejob, "ExtendedJob").Errorf(ctx, "invalid JSON output was emitted for container '%s', secret '%s' cannot be created", instance.GetName(), secretName)
		}

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: instance.GetNamespace(),
			},
		}

		// Persist the output in secret
		secretLabels := ejob.Spec.Output.SecretLabels
		if secretLabels == nil {
			secretLabels = map[string]string{}
		}

		secretLabels[ejv1.LabelPersistentSecretContainer] = c.Name
		if ig, ok := podutil.LookupEnv(c.Env, factory.EnvInstanceGroupName); ok {
			secretLabels[ejv1.LabelInstanceGroup] = ig
		}

		if ejob.Spec.Output.Versioned {

			// Use secretName as versioned secret name prefix: <secretName>-v<version>
			err = r.versionedSecretStore.Create(
				ctx,
				instance.GetNamespace(),
				ejob.GetName(),
				ejob.GetUID(),
				secretName,
				data,
				secretLabels,
				"created by extendedJob")
			if err != nil {
				return errors.Wrapf(err, "could not create persisted output secret for ejob %s", ejob.GetName())
			}
		} else {
			secret.StringData = data
			secret.Labels = secretLabels
			op, err := controllerutil.CreateOrUpdate(ctx, r.client, secret, mutate.SecretMutateFn(secret))
			if err != nil {
				return errors.Wrapf(err, "creating or updating Secret '%s' for ejob %s", secret.Name, ejob.GetName())
			}

			ctxlog.Debugf(ctx, "Output secret '%s' has been %s", secret.Name, op)
		}

	}

	return nil
}
