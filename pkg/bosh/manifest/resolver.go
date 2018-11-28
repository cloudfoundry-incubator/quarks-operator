package manifest

import (
	"context"
	"fmt"

	bdc "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeploymentcontroller/v1alpha1"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Resolver resolves references from CRD to a BOSH manifest
type Resolver interface {
	ResolveCRD(bdc.BOSHDeploymentSpec, string) (*Manifest, error)
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

// ResolveCRD returns manifest referenced by our CRD
func (r *ResolverImpl) ResolveCRD(spec bdc.BOSHDeploymentSpec, namespace string) (*Manifest, error) {
	manifest := &Manifest{}
	var (
		m   string
		err error
		ok  bool
	)
	// TODO for now we only support config map ref
	manifestRef := spec.Manifest

	switch manifestRef.Type {
	case "configMap":
		config := &corev1.ConfigMap{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: manifestRef.Ref, Namespace: namespace}, config)
		if err != nil {
			return manifest, errors.Wrapf(err, "Failed to retrieve configmap '%s/%s' via client.Get", namespace, manifestRef.Ref)
		}
		// unmarshal manifest.data into bosh deployment manifest...
		// TODO re-use LoadManifest() from fissile
		m, ok = config.Data["manifest"]
		if !ok {
			return manifest, fmt.Errorf("configmap doesn't contain manifest key")
		}
	default:
		return manifest, fmt.Errorf("unrecognized manifest type: %s", manifestRef.Type)
	}

	// unmarshal ops.data into bosh ops if exist
	ops := spec.Ops
	if len(ops) == 0 {
		err = yaml.Unmarshal([]byte(m), manifest)
		return manifest, err
	}

	for _, op := range ops {
		switch op.Type {
		case "secret":
			err = r.buildOpsFromSecret(op.Ref, namespace)
			if err != nil {
				return manifest, errors.Wrapf(err, "Failed to build ops from secret %#v", m)
			}
		case "configMap":
			err = r.buildOpsFromConfigMap(op.Ref, namespace)
			if err != nil {
				return manifest, errors.Wrapf(err, "Failed to build ops from config map %#v", m)
			}
		default:
			return manifest, fmt.Errorf("unrecognized ops-ref type: %s", op.Type)
		}
	}

	bytes, err := r.interpolator.Interpolate([]byte(m))
	if err != nil {
		return manifest, errors.Wrapf(err, "Failed to interpolate %#v", m)
	}

	err = yaml.Unmarshal(bytes, manifest)

	return manifest, err
}

func (r *ResolverImpl) buildOpsFromSecret(secretName string, namespace string) error {
	opsSecret := &corev1.Secret{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: secretName, Namespace: namespace}, opsSecret)
	if err != nil {
		return errors.Wrapf(err, "Failed to retrieve secret '%s/%s' via client.Get", namespace, secretName)
	}

	encodedData, ok := opsSecret.Data["ops"]
	if !ok {
		return fmt.Errorf("secert doesn't contain ops key")
	}
	opsData := fmt.Sprintf("%s", encodedData)
	err = r.interpolator.BuildOps([]byte(opsData))
	if err != nil {
		return errors.Wrapf(err, "Failed to build ops: %#v", opsData)
	}

	return nil
}

func (r *ResolverImpl) buildOpsFromConfigMap(configMapName string, namespace string) error {
	opsConfig := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: configMapName, Namespace: namespace}, opsConfig)
	if err != nil {
		return errors.Wrapf(err, "Failed to retrieve config map '%s/%s' via client.Get", namespace, configMapName)
	}

	encodedData, ok := opsConfig.Data["ops"]
	if !ok {
		return fmt.Errorf("config map doesn't contain ops key")
	}
	opsData := fmt.Sprintf("%s", encodedData)
	err = r.interpolator.BuildOps([]byte(opsData))
	if err != nil {
		return errors.Wrapf(err, "Failed to build ops: %#v", opsData)
	}

	return nil
}
