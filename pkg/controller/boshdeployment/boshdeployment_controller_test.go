package boshdeployment_test

import (
	"fmt"
	"testing"

	"code.cloudfoundry.org/cf-operator/pkg/apis"
	fissile "code.cloudfoundry.org/cf-operator/pkg/apis/fissile/v1alpha1"
	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest/manifestfakes"
	cfd "code.cloudfoundry.org/cf-operator/pkg/controller/boshdeployment"
	cfakes "code.cloudfoundry.org/cf-operator/pkg/controller/boshdeployment/fakes"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func init() {
	// Add types to scheme: https://github.com/kubernetes-sigs/controller-runtime/issues/137
	apis.AddToScheme(scheme.Scheme)
}

func TestReconcileFetchInstance(t *testing.T) {
	assert := assert.New(t)

	c := &cfakes.FakeClient{}
	m := &cfakes.FakeManager{}
	m.GetClientReturns(c)

	r := cfd.NewReconciler(m, &manifestfakes.FakeResolver{}, controllerutil.SetControllerReference)

	request := reconcile.Request{NamespacedName: types.NamespacedName{Name: "notfound", Namespace: "default"}}

	c.GetReturns(errors.NewNotFound(schema.GroupResource{}, "not found is requeued"))
	result, err := r.Reconcile(request)
	assert.NoError(err)
	assert.Equal(reconcile.Result{}, result)

	c.GetReturns(errors.NewBadRequest("bad request returns error"))
	result, err = r.Reconcile(request)
	assert.Error(err)
	assert.Contains(err.Error(), "bad request returns error")
}

func TestReconcileResolve(t *testing.T) {
	assert := assert.New(t)

	c := &cfakes.FakeClient{}
	m := &cfakes.FakeManager{}
	m.GetClientReturns(c)
	resolver := &manifestfakes.FakeResolver{}
	resolver.ResolveCRDReturns(nil, fmt.Errorf("resolver error"))

	r := cfd.NewReconciler(m, resolver, controllerutil.SetControllerReference)

	request := reconcile.Request{NamespacedName: types.NamespacedName{Name: "resolver error", Namespace: "default"}}
	_, err := r.Reconcile(request)
	assert.Error(err)
	assert.Contains(err.Error(), "resolver error")
}

func TestReconcileManifestOK(t *testing.T) {
	assert := assert.New(t)

	c := fake.NewFakeClient(
		&fissile.BOSHDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "default",
			},
			Spec: fissile.BOSHDeploymentSpec{},
		},
	)

	m := &cfakes.FakeManager{}
	m.GetClientReturns(c)
	resolver := &manifestfakes.FakeResolver{}
	resolver.ResolveCRDReturns(&bdm.Manifest{}, nil)
	r := cfd.NewReconciler(m, resolver, controllerutil.SetControllerReference)

	request := reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: "default"}}
	_, err := r.Reconcile(request)

	assert.Error(err)
	assert.Contains(err.Error(), "manifest is missing instance groups")
}

func TestReconcileSetControllerReference(t *testing.T) {
	assert := assert.New(t)

	c := fake.NewFakeClient(
		&fissile.BOSHDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "default",
			},
			Spec: fissile.BOSHDeploymentSpec{},
		},
	)

	m := &cfakes.FakeManager{}
	m.GetClientReturns(c)
	manifest := &bdm.Manifest{
		InstanceGroups: []bdm.InstanceGroup{
			bdm.InstanceGroup{Name: "fakepod"},
		},
	}
	resolver := &manifestfakes.FakeResolver{}
	resolver.ResolveCRDReturns(manifest, nil)
	r := cfd.NewReconciler(m, resolver, func(owner, object metav1.Object, scheme *runtime.Scheme) error {
		return fmt.Errorf("failed to set reference")
	})

	request := reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: "default"}}
	_, err := r.Reconcile(request)

	assert.Error(err)
	assert.Contains(err.Error(), "failed to set reference")
}
