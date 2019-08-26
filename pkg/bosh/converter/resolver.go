package converter

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/names"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/versionedsecretstore"
)

// Resolver interface to provide a BOSH manifest resolved references from bdpl CRD
type Resolver interface {
	WithOpsManifest(instance *bdv1.BOSHDeployment, namespace string) (*bdm.Manifest, []string, error)
}

// ResolverImpl resolves references from bdpl CRD to a BOSH manifest
type ResolverImpl struct {
	client               client.Client
	versionedSecretStore versionedsecretstore.VersionedSecretStore
	newInterpolatorFunc  NewInterpolatorFunc
}

// NewInterpolatorFunc returns a fresh Interpolator
type NewInterpolatorFunc func() Interpolator

// NewResolver constructs a resolver
func NewResolver(client client.Client, f NewInterpolatorFunc) *ResolverImpl {
	return &ResolverImpl{
		client:               client,
		newInterpolatorFunc:  f,
		versionedSecretStore: versionedsecretstore.NewVersionedSecretStore(client),
	}
}

// DesiredManifest reads the versioned secret created by the variable interpolation job
// and unmarshals it into a Manifest object
func (r *ResolverImpl) DesiredManifest(ctx context.Context, boshDeploymentName, namespace string) (*bdm.Manifest, error) {
	// unversioned desired manifest name
	secretName := names.DesiredManifestName(boshDeploymentName, "")

	secret, err := r.versionedSecretStore.Latest(ctx, namespace, secretName)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read latest versioned secret %s for bosh deployment %s", secretName, boshDeploymentName)
	}

	manifestData := secret.Data["manifest.yaml"]

	manifest, err := bdm.LoadYAML(manifestData)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to unmarshal manifest from secret %s for boshdeployment %s", secretName, boshDeploymentName)
	}

	return manifest, nil
}

// WithOpsManifest returns manifest and a list of implicit variables referenced by our bdpl CRD
// The resulting manifest has variables interpolated and ops files applied.
// It is the 'with-ops' manifest.
func (r *ResolverImpl) WithOpsManifest(instance *bdv1.BOSHDeployment, namespace string) (*bdm.Manifest, []string, error) {
	interpolator := r.newInterpolatorFunc()
	spec := instance.Spec
	var (
		m   string
		err error
	)

	m, err = r.resourceData(namespace, spec.Manifest.Type, spec.Manifest.Name, bdv1.ManifestSpecName)
	if err != nil {
		return nil, []string{}, errors.Wrapf(err, "Interpolation failed for bosh deployment %s", instance.GetName())
	}

	// Interpolate manifest with ops
	ops := spec.Ops

	for _, op := range ops {
		opsData, err := r.resourceData(namespace, op.Type, op.Name, bdv1.OpsSpecName)
		if err != nil {
			return nil, []string{}, errors.Wrapf(err, "Failed to get resource data for interpolation of bosh deployment '%s' and ops '%s'", instance.GetName(), op.Name)
		}
		err = interpolator.BuildOps([]byte(opsData))
		if err != nil {
			return nil, []string{}, errors.Wrapf(err, "Interpolation failed for bosh deployment '%s' and ops '%s'", instance.GetName(), op.Name)
		}
	}

	bytes := []byte(m)
	if len(ops) != 0 {
		bytes, err = interpolator.Interpolate([]byte(m))
		if err != nil {
			return nil, []string{}, errors.Wrapf(err, "Failed to interpolate '%s' in interpolation task", instance.Name)
		}
	}

	// Reload the manifest after interpolation, and apply implicit variables
	manifest, err := bdm.LoadYAML(bytes)
	if err != nil {
		return nil, []string{}, errors.Wrapf(err, "Loading yaml failed in interpolation task after applying ops %#v", m)
	}
	m = string(bytes)

	// Interpolate implicit variables
	vars, err := manifest.ImplicitVariables()
	if err != nil {
		return nil, []string{}, errors.Wrapf(err, "failed to list implicit variables")
	}

	varSecrets := make([]string, len(vars))
	for i, v := range vars {
		varSecretName := names.CalculateSecretName(names.DeploymentSecretTypeVariable, instance.GetName(), v)
		varData, err := r.resourceData(namespace, bdv1.SecretReference, varSecretName, bdv1.ImplicitVariableKeyName)
		if err != nil {
			return nil, varSecrets, errors.Wrapf(err, "failed to load secret for variable '%s'", v)
		}

		varSecrets[i] = varSecretName
		m = strings.Replace(m, fmt.Sprintf("((%s))", v), varData, -1)
	}

	manifest, err = bdm.LoadYAML([]byte(m))
	if err != nil {
		return nil, varSecrets, errors.Wrapf(err, "failed to load yaml after interpolating implicit variables %#v", m)
	}

	// Apply addons
	err = manifest.ApplyAddons()
	if err != nil {
		return nil, varSecrets, errors.Wrapf(err, "failed to apply addons")
	}

	return manifest, varSecrets, err
}

// resourceData resolves different manifest reference types and returns the resource's data
func (r *ResolverImpl) resourceData(namespace string, resType bdv1.ReferenceType, name string, key string) (string, error) {
	var (
		data string
		ok   bool
	)

	switch resType {
	case bdv1.ConfigMapReference:
		opsConfig := &corev1.ConfigMap{}
		err := r.client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, opsConfig)
		if err != nil {
			return data, errors.Wrapf(err, "failed to retrieve %s from configmap '%s/%s' via client.Get", key, namespace, name)
		}
		data, ok = opsConfig.Data[key]
		if !ok {
			return data, fmt.Errorf("configMap '%s/%s' doesn't contain key %s", namespace, name, key)
		}
	case bdv1.SecretReference:
		opsSecret := &corev1.Secret{}
		err := r.client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, opsSecret)
		if err != nil {
			return data, errors.Wrapf(err, "failed to retrieve %s from secret '%s/%s' via client.Get", key, namespace, name)
		}
		encodedData, ok := opsSecret.Data[key]
		if !ok {
			return data, fmt.Errorf("secret '%s/%s' doesn't contain key %s", namespace, name, key)
		}
		data = string(encodedData)
	case bdv1.URLReference:
		httpResponse, err := http.Get(name)
		if err != nil {
			return data, errors.Wrapf(err, "failed to resolve %s from url '%s' via http.Get", key, name)
		}
		body, err := ioutil.ReadAll(httpResponse.Body)
		if err != nil {
			return data, errors.Wrapf(err, "failed to read %s response body '%s' via ioutil", key, name)
		}
		data = string(body)
	default:
		return data, fmt.Errorf("unrecognized %s ref type %s", key, name)
	}

	return data, nil
}
