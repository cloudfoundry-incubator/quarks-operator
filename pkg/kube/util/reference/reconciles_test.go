package reference_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	estsv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers"
	cfakes "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/fakes"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/reference"
	"code.cloudfoundry.org/cf-operator/testing"
)

var _ = Describe("GetReconciles", func() {
	Context("when getting reconciles for Ests", func() {
		var (
			ests    estsv1.ExtendedStatefulSet
			manager *cfakes.FakeManager

			configMap1 corev1.ConfigMap
			configMap2 corev1.ConfigMap
			secret1    corev1.Secret
			secret2    corev1.Secret
			env        testing.Catalog
			client     client.Client
		)

		BeforeEach(func() {
			controllers.AddToScheme(scheme.Scheme)
			manager = &cfakes.FakeManager{}
			manager.GetSchemeReturns(scheme.Scheme)

			ests = env.OwnedReferencesExtendedStatefulSet("foo")
			configMap1 = env.DefaultConfigMap("example1")
			configMap2 = env.DefaultConfigMap("example2")
			secret1 = env.DefaultSecret("example1")
			secret2 = env.DefaultSecret("example2")
		})

		JustBeforeEach(func() {
			client = fake.NewFakeClient(
				&ests,
				&configMap1,
				&configMap2,
				&secret1,
				&secret2,
			)
		})

		Context("when UpdateOnConfigChange is true", func() {
			BeforeEach(func() {
				ests.Spec.UpdateOnConfigChange = true
			})

			It("triggers a reconcile when a ConfigRef changes", func() {
				requests, err := reference.GetReconciles(context.Background(), client, reference.ReconcileForExtendedStatefulSet, &configMap1)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(requests)).To(Equal(1))
			})

			It("triggers a reconcile when a ConfigKeyRef changes", func() {
				requests, err := reference.GetReconciles(context.Background(), client, reference.ReconcileForExtendedStatefulSet, &configMap2)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(requests)).To(Equal(1))
			})

			It("triggers a reconcile when a SecretRef changes", func() {
				requests, err := reference.GetReconciles(context.Background(), client, reference.ReconcileForExtendedStatefulSet, &secret1)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(requests)).To(Equal(1))
			})

			It("triggers a reconcile when a SecretKeyRef changes", func() {
				requests, err := reference.GetReconciles(context.Background(), client, reference.ReconcileForExtendedStatefulSet, &secret2)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(requests)).To(Equal(1))
			})
		})

		Context("when UpdateOnConfigChange is false", func() {
			BeforeEach(func() {
				ests.Spec.UpdateOnConfigChange = false
			})

			It("doesn't trigger a reconcile when a referenced configmap changes", func() {
				requests, err := reference.GetReconciles(context.Background(), client, reference.ReconcileForExtendedStatefulSet, &configMap1)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(requests)).To(Equal(0))
			})
		})
	})
})
