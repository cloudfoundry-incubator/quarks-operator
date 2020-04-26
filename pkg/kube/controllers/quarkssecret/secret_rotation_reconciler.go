package quarkssecret

import (
	"context"
	"encoding/json"

	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	qsv1a1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/quarkssecret/v1alpha1"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	"code.cloudfoundry.org/quarks-utils/pkg/pointers"
)

// NewSecretRotationReconciler returns a new ReconcileQuarksSecret
func NewSecretRotationReconciler(ctx context.Context, config *config.Config, mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileSecretRotation{
		ctx:    ctx,
		config: config,
		client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
	}
}

// ReconcileSecretRotation reconciles an QuarksSecret object
type ReconcileSecretRotation struct {
	ctx    context.Context
	client client.Client
	scheme *runtime.Scheme
	config *config.Config
}

// Reconcile reads that state of the cluster and trigger secret rotation for
// all listed deployments.
func (r *ReconcileSecretRotation) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	instance := &corev1.ConfigMap{}

	// Set the ctx to be Background, as the top-level context for incoming requests.
	ctx, cancel := context.WithTimeout(r.ctx, r.config.CtxTimeOut)
	defer cancel()

	ctxlog.Infof(ctx, "Reconciling QuarksSecret rotation '%s'", request.NamespacedName)
	err := r.client.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		if apierrors.IsNotFound(err) {
			ctxlog.Info(ctx, "Skip reconcile: CRD not found")
			return reconcile.Result{}, nil
		}
		ctxlog.Info(ctx, "Error reading the object")
		return reconcile.Result{}, errors.Wrap(err, "Error reading quarksSecret")
	}

	data, found := instance.Data[qsv1a1.RotateQSecretListName]
	if !found {
		ctxlog.Debugf(ctx, "QuarksSecret rotation config didn't list any names, key %s not found", qsv1a1.RotateQSecretListName)
		return reconcile.Result{}, nil
	}

	names := []string{}
	err = json.Unmarshal([]byte(data), &names)
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "Error un-marshalling list of secrets to rotate from '%s'", request.NamespacedName)
	}

	for _, name := range names {
		qsec := &qsv1a1.QuarksSecret{}
		err := r.client.Get(ctx, types.NamespacedName{Name: name, Namespace: instance.Namespace}, qsec)
		if err != nil {
			ctxlog.Errorf(ctx, "Error getting QuarksSecret the object '%s', skipping secret rotation", qsec.GetNamespacedName())
			continue
		}

		// skip manual secrets or the ones that have not yet been generated
		if qsec.Status.Generated != nil && !*qsec.Status.Generated {
			ctxlog.Debugf(ctx, "QuarksSecret '%s' cannot be rotated, it was not generated", qsec.GetNamespacedName())
			continue
		}

		qsec.Status.Generated = pointers.Bool(false)
		ctxlog.Debugf(ctx, "QuarksSecret '%s' cannot be rotated, it was not yet generated", qsec.GetNamespacedName())

		err = r.client.Status().Update(ctx, qsec)
		if err != nil {
			return reconcile.Result{}, errors.Wrap(err, "Error updating QuarksSecret status")
		}
	}

	return reconcile.Result{}, nil
}
