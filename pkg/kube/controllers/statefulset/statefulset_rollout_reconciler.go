package statefulset

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"code.cloudfoundry.org/quarks-utils/pkg/meltdown"

	"code.cloudfoundry.org/cf-operator/pkg/kube/apis"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	"code.cloudfoundry.org/quarks-utils/pkg/pointers"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	crc "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	rolloutStatePending       = "Pending"
	rolloutStateCanary        = "Canary"
	rolloutStateRollout       = "Rollout"
	rolloutStateDone          = "Done"
	rolloutStateFailed        = "Failed"
	rolloutStateCanaryUpscale = "CanaryUpscale"
)

var (
	// AnnotationCanaryRolloutEnabled if set to "true" canary behaviour is desired
	AnnotationCanaryRolloutEnabled = fmt.Sprintf("%s/canary-rollout-enabled", apis.GroupName)
	// AnnotationCanaryRollout is the state of the canary rollout of the stateful set
	AnnotationCanaryRollout = fmt.Sprintf("%s/canary-rollout", apis.GroupName)
	// AnnotationCanaryWatchTime is the max time for the canary update
	AnnotationCanaryWatchTime = fmt.Sprintf("%s/canary-watch-time-ms", apis.GroupName)
	// AnnotationUpdateWatchTime is the max time for the complete update
	AnnotationUpdateWatchTime = fmt.Sprintf("%s/update-watch-time-ms", apis.GroupName)
	// AnnotationUpdateStartTime is the timestamp when the update started
	AnnotationUpdateStartTime = fmt.Sprintf("%s/update-start-time", apis.GroupName)
)

// NewStatefulSetRolloutReconciler returns a new reconcile.Reconciler
func NewStatefulSetRolloutReconciler(ctx context.Context, config *config.Config, mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileStatefulSetRollout{
		ctx:    ctx,
		config: config,
		client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
	}
}

// ReconcileStatefulSetRollout reconciles an QuarksStatefulSet object when references changes
type ReconcileStatefulSetRollout struct {
	ctx    context.Context
	client crc.Client
	scheme *runtime.Scheme
	config *config.Config
}

// Reconcile cleans up old versions and volumeManagement statefulSet of the QuarksStatefulSet
func (r *ReconcileStatefulSetRollout) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Set the ctx to be Background, as the top-level context for incoming requests.
	ctx, cancel := context.WithTimeout(r.ctx, r.config.CtxTimeOut)
	defer cancel()

	ctxlog.Debug(ctx, "Reconciling StatefulSet ", request.NamespacedName)

	statefulSet := appsv1.StatefulSet{}

	err := r.client.Get(ctx, request.NamespacedName, &statefulSet)
	if err != nil {
		ctxlog.Error(ctx, "StatefulSet not found ", request.NamespacedName)
		return reconcile.Result{}, err
	}

	if meltdown.NewAnnotationWindow(r.config.MeltdownDuration, statefulSet.Annotations).Contains(time.Now()) {
		ctxlog.WithEvent(&statefulSet, "Meltdown").Debugf(ctx, "Resource '%s' is in meltdown, requeue reconcile after %s", statefulSet.Name, r.config.MeltdownRequeueAfter)
		return reconcile.Result{RequeueAfter: r.config.MeltdownRequeueAfter}, nil
	}

	var status = statefulSet.Annotations[AnnotationCanaryRollout]
	if status == rolloutStateFailed || status == rolloutStateDone {
		return reconcile.Result{}, nil
	}

	var newStatus = status
	dirty := false
	var oldPartition int32
	if statefulSet.Spec.UpdateStrategy.RollingUpdate.Partition == nil {
		oldPartition = -1
		ctxlog.Error(ctx, "StatefulSet with unexpected partition = nil found (webhook should prevent this)", request.NamespacedName)
	} else {
		oldPartition = *statefulSet.Spec.UpdateStrategy.RollingUpdate.Partition
	}

	if timedOut, err := r.failIfTimedOut(ctx, statefulSet, AnnotationUpdateWatchTime); timedOut || err != nil {
		return reconcile.Result{}, err
	}

	var resultWithRetrigger reconcile.Result
	resultWithRetrigger.RequeueAfter = getTimeOut(ctx, statefulSet, AnnotationUpdateWatchTime)

	switch status {
	case rolloutStateCanaryUpscale:
		if statefulSet.Status.Replicas == *statefulSet.Spec.Replicas && statefulSet.Status.ReadyReplicas == *statefulSet.Spec.Replicas {
			if *statefulSet.Spec.UpdateStrategy.RollingUpdate.Partition == 0 {
				newStatus = rolloutStateDone
			} else {
				(*statefulSet.Spec.UpdateStrategy.RollingUpdate.Partition)--
				newStatus = rolloutStateRollout
			}
		}
	case rolloutStateCanary:
		if getTimeOut(ctx, statefulSet, AnnotationCanaryWatchTime) < 0 {
			newStatus = rolloutStateFailed
			break
		}
		fallthrough
	case rolloutStateRollout:
		if resultWithRetrigger.RequeueAfter > time.Minute {
			resultWithRetrigger.RequeueAfter = time.Minute
		}
		ready, err := partitionPodIsReadyAndUpdated(ctx, r.client, &statefulSet)
		if err != nil {
			return reconcile.Result{}, err
		}
		if *statefulSet.Spec.UpdateStrategy.RollingUpdate.Partition == 0 {
			if ready {
				newStatus = rolloutStateDone
			}
			break
		}
		if !ready {
			break
		}
		(*statefulSet.Spec.UpdateStrategy.RollingUpdate.Partition)--
		dirty = true
		newStatus = rolloutStateRollout
	case rolloutStatePending:
		if statefulSet.Status.Replicas < *statefulSet.Spec.Replicas {
			newStatus = rolloutStateCanaryUpscale
			resultWithRetrigger.RequeueAfter = getTimeOut(ctx, statefulSet, AnnotationUpdateWatchTime)
		} else {
			resultWithRetrigger.RequeueAfter = getTimeOut(ctx, statefulSet, AnnotationCanaryWatchTime)
			newStatus = rolloutStateCanary
			(*statefulSet.Spec.UpdateStrategy.RollingUpdate.Partition)--
			dirty = true
		}
	}
	statusChanged := newStatus != statefulSet.Annotations[AnnotationCanaryRollout]
	if statusChanged {
		statefulSet.Annotations[AnnotationCanaryRollout] = newStatus
		dirty = true
	}
	if dirty {
		if err = r.updateWithPartitionMove(ctx, statefulSet, oldPartition); err != nil {
			ctxlog.Debug(ctx, "Error updating StatefulSet ", request.NamespacedName, err)
			return reconcile.Result{}, err
		}
	}
	return resultWithRetrigger, nil
}

func (r *ReconcileStatefulSetRollout) failIfTimedOut(ctx context.Context, statefulSet appsv1.StatefulSet, timeout string) (bool, error) {
	if getTimeOut(ctx, statefulSet, timeout) < 0 {
		statefulSet.Annotations[AnnotationCanaryRollout] = rolloutStateFailed
		if err := r.updateStatefulSet(ctx, &statefulSet); err != nil {
			ctxlog.Debug(ctx, "Error updating StatefulSet ", statefulSet.Name, err)
			return true, err
		}
		return true, nil
	}
	return false, nil
}

func (r *ReconcileStatefulSetRollout) updateWithPartitionMove(ctx context.Context, statefulset appsv1.StatefulSet, oldPartition int32) error {
	err := r.updateStatefulSet(ctx, &statefulset)
	if err != nil {
		return err
	}

	if oldPartition != *statefulset.Spec.UpdateStrategy.RollingUpdate.Partition {
		err = CleanupNonReadyPod(ctx, r.client, &statefulset, *statefulset.Spec.UpdateStrategy.RollingUpdate.Partition)
		if err != nil {
			return err
		}
	}
	return nil

}

func getTimeOut(ctx context.Context, statefulSet appsv1.StatefulSet, watchTimeAnnotation string) time.Duration {
	watchTimeStr, ok := statefulSet.Annotations[watchTimeAnnotation]
	if !ok || watchTimeStr == "" {
		return 0 // never timeout
	}
	watchTime, err := strconv.Atoi(watchTimeStr)
	if err != nil {
		ctxlog.Errorf(ctx, "Invalid annotation %s: %s", watchTimeAnnotation, statefulSet.Annotations[watchTimeAnnotation])
		return -1
	}
	updateStartTimeUnix, err := strconv.ParseInt(statefulSet.Annotations[AnnotationUpdateStartTime], 10, 64)
	if err != nil {
		ctxlog.Errorf(ctx, "Invalid annotation %s: %s", AnnotationUpdateStartTime, statefulSet.Annotations[AnnotationUpdateStartTime])
		return -1
	}
	updateStartTime := time.Unix(updateStartTimeUnix, 0)
	timeLeft := time.Until(updateStartTime.Add(time.Millisecond * time.Duration(watchTime)))
	if timeLeft == 0 { //differ from 'never timeout'
		timeLeft = -1
	}
	return timeLeft
}

func (r *ReconcileStatefulSetRollout) updateStatefulSet(ctx context.Context, statefulSet *appsv1.StatefulSet) error {
	meltdown.SetLastReconcile(&statefulSet.ObjectMeta, time.Now())

	partition := *statefulSet.Spec.UpdateStrategy.RollingUpdate.Partition
	state := statefulSet.Annotations[AnnotationCanaryRollout]
	_, err := controllerutil.CreateOrUpdate(ctx, r.client, statefulSet, func() error {
		statefulSet.Spec.UpdateStrategy.RollingUpdate.Partition = pointers.Int32(partition)
		statefulSet.Annotations[AnnotationCanaryRollout] = state
		return nil
	})
	if err != nil {
		if err != nil {
			statusError, ok := err.(*errors.StatusError)
			if ok && statusError.Status().Code == 409 {
				ctxlog.Debug(ctx, "Conflict while updating stateful set: ", err.Error())
				return err
			}
			ctxlog.Errorf(ctx, "Error while updating stateful set: ", err.Error())
			return err
		}
		ctxlog.Errorf(ctx, "Error while updating stateful set: ", err.Error())
		return err
	}
	ctxlog.Debugf(ctx, "StatefulSet %s/%s updated to state Done ", statefulSet.Namespace, statefulSet.Name)
	return nil
}

func partitionPodIsReadyAndUpdated(ctx context.Context, client crc.Client, statefulSet *appsv1.StatefulSet) (bool, error) {
	ready := false
	updated := false
	if statefulSet.Spec.UpdateStrategy.RollingUpdate != nil {
		pod, podReady, err := getPodWithIndex(ctx, client, statefulSet, *statefulSet.Spec.UpdateStrategy.RollingUpdate.Partition)
		if err != nil {
			ctxlog.Debug(ctx, "Error calling GetNoneReadyPod ", statefulSet.Namespace, "/", statefulSet.Name, err)
			return false, err
		}
		if podReady {
			ready = true
			updated = pod.Labels[appsv1.StatefulSetRevisionLabel] == statefulSet.Status.UpdateRevision
		}
	}
	return ready && updated, nil
}
