package manifest

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	bdc "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/names"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/versionedsecretstore"
)

// Resolver resolves references from bdpl CRD to a BOSH manifest
type Resolver struct {
	client               client.Client
	versionedSecretStore versionedsecretstore.VersionedSecretStore
	newInterpolatorFunc  func() Interpolator
}

// NewInterpolatorFunc returns a fresh Interpolator
type NewInterpolatorFunc func() Interpolator

// NewResolver constructs a resolver
func NewResolver(client client.Client, f NewInterpolatorFunc) *Resolver {
	return &Resolver{
		client:               client,
		newInterpolatorFunc:  f,
		versionedSecretStore: versionedsecretstore.NewVersionedSecretStore(client),
	}
}

// WithOpsManifest returns manifest referenced by our bdpl CRD
// The resulting manifest has variables interpolated and ops files applied.
// It is the 'with-ops' manifest.
func (r *Resolver) WithOpsManifest(instance *bdc.BOSHDeployment, namespace string) (*Manifest, error) {
	interpolator := r.newInterpolatorFunc()
	spec := instance.Spec
	manifest := &Manifest{}
	var (
		m   string
		err error
	)

	m, err = r.getRefData(namespace, spec.Manifest.Type, spec.Manifest.Ref, bdc.ManifestSpecName)
	if err != nil {
		return manifest, err
	}

	// Get the deployment name from the manifest
	manifest, err = LoadYAML([]byte(m))
	if err != nil {
		return manifest, errors.Wrapf(err, "failed to unmarshal manifest")
	}

	// Interpolate implicit variables
	vars := r.ImplicitVariables(manifest, m)
	for _, v := range vars {
		varData, err := r.getRefData(namespace, bdc.SecretType, names.CalculateSecretName(names.DeploymentSecretTypeImplicitVariable, instance.GetName(), v), bdc.ImplicitVariableKeyName)
		if err != nil {
			return manifest, errors.Wrapf(err, "failed to load secret for variable '%s'", v)
		}

		m = strings.Replace(m, fmt.Sprintf("((%s))", v), varData, -1)
	}

	// Interpolate manifest with ops if exist
	ops := spec.Ops
	if len(ops) == 0 {
		err = yaml.Unmarshal([]byte(m), manifest)
		return manifest, err
	}

	for _, op := range ops {
		opsData, err := r.getRefData(namespace, op.Type, op.Ref, bdc.OpsSpecName)
		if err != nil {
			return manifest, err
		}
		err = interpolator.BuildOps([]byte(opsData))
		if err != nil {
			return manifest, errors.Wrapf(err, "failed to build ops with: %#v", opsData)
		}
	}

	bytes, err := interpolator.Interpolate([]byte(m))
	if err != nil {
		return manifest, errors.Wrapf(err, "failed to interpolate %#v", m)
	}

	manifest, err = LoadYAML(bytes)

	return manifest, err
}

// getRefData resolves different manifest reference types and returns manifest data
func (r *Resolver) getRefData(namespace string, manifestType string, manifestRef string, refKey string) (string, error) {
	var (
		refData string
		ok      bool
	)

	switch manifestType {
	case bdc.ConfigMapType:
		opsConfig := &corev1.ConfigMap{}
		err := r.client.Get(context.TODO(), types.NamespacedName{Name: manifestRef, Namespace: namespace}, opsConfig)
		if err != nil {
			return refData, errors.Wrapf(err, "failed to retrieve %s from configmap '%s/%s' via client.Get", refKey, namespace, manifestRef)
		}
		refData, ok = opsConfig.Data[refKey]
		if !ok {
			return refData, fmt.Errorf("configMap '%s/%s' doesn't contain key %s", namespace, manifestRef, refKey)
		}
	case bdc.SecretType:
		opsSecret := &corev1.Secret{}
		err := r.client.Get(context.TODO(), types.NamespacedName{Name: manifestRef, Namespace: namespace}, opsSecret)
		if err != nil {
			return refData, errors.Wrapf(err, "failed to retrieve %s from secret '%s/%s' via client.Get", refKey, namespace, manifestRef)
		}
		encodedData, ok := opsSecret.Data[refKey]
		if !ok {
			return refData, fmt.Errorf("secret '%s/%s' doesn't contain key %s", namespace, manifestRef, refKey)
		}
		refData = string(encodedData)
	case bdc.URLType:
		httpResponse, err := http.Get(manifestRef)
		if err != nil {
			return refData, errors.Wrapf(err, "failed to resolve %s from url '%s' via http.Get", refKey, manifestRef)
		}
		body, err := ioutil.ReadAll(httpResponse.Body)
		if err != nil {
			return refData, errors.Wrapf(err, "failed to read %s response body '%s' via ioutil", refKey, manifestRef)
		}
		refData = string(body)
	default:
		return refData, fmt.Errorf("unrecognized %s ref type %s", refKey, manifestRef)
	}

	return refData, nil
}

// ImplicitVariables returns a list of all implicit variables in a manifest
func (r *Resolver) ImplicitVariables(m *Manifest, rawManifest string) []string {
	varMap := make(map[string]bool)

	// Collect all variables
	varRegexp := regexp.MustCompile(`\(\((!?[-/\.\w\pL]+)\)\)`)
	for _, match := range varRegexp.FindAllStringSubmatch(rawManifest, -1) {
		// Remove subfields from the match, e.g. ca.private_key -> ca
		fieldRegexp := regexp.MustCompile(`[^\.]+`)
		main := fieldRegexp.FindString(match[1])

		varMap[main] = true
	}

	// Remove the explicit ones
	for _, v := range m.Variables {
		varMap[v.Name] = false
	}

	names := []string{}
	for k, v := range varMap {
		if v {
			names = append(names, k)
		}
	}

	return names
}
