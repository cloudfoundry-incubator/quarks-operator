package manifest

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	bdc "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
)

// Resolver resolves references from CRD to a BOSH manifest
type Resolver interface {
	ResolveManifest(bdc.BOSHDeploymentSpec, string) (*Manifest, error)
}

// ResolverImpl implements Resolver interface
type ResolverImpl struct {
	client       client.Client
	interpolator Interpolator
}

// NewResolver constructs a resolver
func NewResolver(client client.Client, interpolator Interpolator) *ResolverImpl {
	return &ResolverImpl{client: client, interpolator: interpolator}
}

// ResolveManifest returns manifest referenced by our CRD
func (r *ResolverImpl) ResolveManifest(spec bdc.BOSHDeploymentSpec, namespace string) (*Manifest, error) {
	manifest := &Manifest{}
	var (
		m   string
		err error
	)

	m, err = r.getRefData(namespace, spec.Manifest.Type, spec.Manifest.Ref, bdc.ManifestSpecName)
	if err != nil {
		return manifest, err
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
		err = r.interpolator.BuildOps([]byte(opsData))
		if err != nil {
			return manifest, errors.Wrapf(err, "failed to build ops with: %#v", opsData)
		}
	}

	bytes, err := r.interpolator.Interpolate([]byte(m))
	if err != nil {
		return manifest, errors.Wrapf(err, "failed to interpolate %#v", m)
	}

	err = yaml.Unmarshal(bytes, manifest)

	return manifest, err
}

// getRefData resolves different manifest reference types and returns manifest data
func (r *ResolverImpl) getRefData(namespace string, manifestType string, manifestRef string, refKey string) (string, error) {
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
			return refData, fmt.Errorf("secert '%s/%s' doesn't contain key %s", namespace, manifestRef, refKey)
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
