package manifest_test

import (
	"testing"

	fissile "code.cloudfoundry.org/cf-operator/pkg/apis/fissile/v1alpha1"
	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestResolveCRD(t *testing.T) {
	assert := assert.New(t)

	client := fake.NewFakeClient(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "default",
			},
			Data: map[string]string{"manifest": "---"},
		},
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "invalid_yaml",
				Namespace: "default",
			},
			Data: map[string]string{"manifest": "!yaml"},
		},
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "missing_key",
				Namespace: "default",
			},
			Data: map[string]string{},
		},
	)

	resolver := bdm.NewResolver(client)

	spec := fissile.CFDeploymentSpec{ManifestRef: "foo"}
	manifest, err := resolver.ResolveCRD(spec, "default")

	assert.NoError(err)
	assert.NotNil(manifest)
	assert.Equal(0, len(manifest.InstanceGroups))

	spec = fissile.CFDeploymentSpec{ManifestRef: "bar"}
	manifest, err = resolver.ResolveCRD(spec, "default")
	assert.Error(err)
	assert.Contains(err.Error(), "configmaps \"bar\" not found")

	spec = fissile.CFDeploymentSpec{ManifestRef: "missing_key"}
	manifest, err = resolver.ResolveCRD(spec, "default")
	assert.Error(err)
	assert.Contains(err.Error(), "configmap doesn't contain manifest key")

	spec = fissile.CFDeploymentSpec{ManifestRef: "invalid_yaml"}
	manifest, err = resolver.ResolveCRD(spec, "default")
	assert.Error(err)
	assert.Contains(err.Error(), "yaml: unmarshal errors")
}
