// Package testing contains methods to create test data. It's a seaparate
// package to avoid import cycles. Helper functions can be found in the package
// `testhelper`.
package testing

import (
	"context"
	"os"
	"time"

	"github.com/spf13/afero"
	yaml "gopkg.in/yaml.v2"
	"k8s.io/api/apps/v1beta2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/cf-operator/pkg/credsgen"
	bdcv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	esv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedsecret/v1alpha1"
	essv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"k8s.io/apimachinery/pkg/labels"
)

// NewContext returns a non-nil empty context, for usage when it is unclear
// which context to use.  Mostly used in tests.
func NewContext() context.Context {
	return context.TODO()
}

// Catalog provides several instances for tests
type Catalog struct{}

// DefaultConfig for tests
func (c *Catalog) DefaultConfig() *config.Config {
	return &config.Config{
		CtxTimeOut:        10 * time.Second,
		Namespace:         "default",
		WebhookServerHost: "foo.com",
		WebhookServerPort: 1234,
		Fs:                afero.NewMemMapFs(),
	}
}

// DefaultBOSHManifest for tests
func (c *Catalog) DefaultBOSHManifest() manifest.Manifest {
	m := manifest.Manifest{}
	source := `name: foo-deployment
stemcells:
- alias: default
  os: opensuse-42.3
  version: 28.g837c5b3-30.263-7.0.0_234.gcd7d1132
instance_groups:
- name: redis-slave
  instances: 2
  lifecycle: errand
  azs: [z1, z2]
  jobs:
  - name: redis-server
    release: redis
    properties: {}
  vm_type: medium
  stemcell: default
  persistent_disk_type: medium
  networks:
  - name: default
  properties:
    foo:
      app_domain: "((app_domain))"
    bosh_containerization:
      ports:
      - name: "redis"
        protocol: "TCP"
        internal: 6379
- name: diego-cell
  azs:
  - z1
  - z2
  instances: 2
  lifecycle: service
  vm_type: small-highmem
  vm_extensions:
  - 100GB_ephemeral_disk
  stemcell: default
  networks:
  - name: default
  jobs:
  - name: cflinuxfs3-rootfs-setup
    release: cflinuxfs3
    properties:
      foo:
        domain: "((system_domain))"
      bosh_containerization:
        ports:
        - name: "rep-server"
          protocol: "TCP"
          internal: 1801
variables:
- name: "adminpass"
  type: "password"
  options: {is_ca: true, common_name: "some-ca"}
releases:
- name: cflinuxfs3
  version: 0.62.0
  url: hub.docker.com/cfcontainerization
  sha1: 6466c44827c3493645ca34b084e7c21de23272b4
  stemcell:
    os: opensuse-15.0
    version: 28.g837c5b3-30.263-7.0.0_233.gde0accd0
- name: redis
  version: 36.15.0
  url: hub.docker.com/cfcontainerization
  sha1: 6466c44827c3493645ca34b084e7c21de23272b4`
	err := yaml.Unmarshal([]byte(source), &m)
	if err != nil {
		panic(err)
	}
	return m
}

// ElaboratedBOSHManifest for data gathering tests
func (c *Catalog) ElaboratedBOSHManifest() *manifest.Manifest {
	m := &manifest.Manifest{}
	source := `name: foo-deployment
stemcells:
- alias: default
  os: opensuse-42.3
  version: 28.g837c5b3-30.263-7.0.0_234.gcd7d1132
instance_groups:
- name: redis-slave
  instances: 2
  lifecycle: errand
  azs: [z1, z2]
  jobs:
  - name: redis-server
    release: redis
    properties:
      password: foobar
    provides:
      redis: {as: redis-server}
  - name: cflinuxfs3-rootfs-setup
    release: cflinuxfs3
  vm_type: medium
  stemcell: default
  persistent_disk_type: medium
  networks:
  - name: default
- name: diego-cell
  azs:
  - z1
  - z2
  instances: 2
  lifecycle: service
  vm_type: small-highmem
  vm_extensions:
  - 100GB_ephemeral_disk
  stemcell: default
  networks:
  - name: default
  jobs:
  - name: cflinuxfs3-rootfs-setup
    release: cflinuxfs3
variables:
- name: "adminpass"
  type: "password"
  options: {is_ca: true, common_name: "some-ca"}
releases:
- name: cflinuxfs3
  version: 0.62.0
  url: hub.docker.com/cfcontainerization
  sha1: 6466c44827c3493645ca34b084e7c21de23272b4
  stemcell:
    os: opensuse-15.0
    version: 28.g837c5b3-30.263-7.0.0_233.gde0accd0
- name: redis
  version: 36.15.0
  url: hub.docker.com/cfcontainerization
  sha1: 6466c44827c3493645ca34b084e7c21de23272b4`
	err := yaml.Unmarshal([]byte(source), m)
	if err != nil {
		panic(err)
	}
	return m
}

// BOSHManifestWithProviderAndConsumer for data gathering tests
func (c *Catalog) BOSHManifestWithProviderAndConsumer() *manifest.Manifest {
	m := &manifest.Manifest{}
	source := `---
name: cf
manifest_version: v7.7.0
instance_groups:
- name: doppler
  azs:
  - z1
  - z2
  instances: 4
  vm_type: minimal
  stemcell: default
  networks:
  - name: default
  jobs:
  - name: doppler
    release: loggregator
    provides:
      doppler: {as: doppler, shared: true}
    properties:
      doppler:
        grpc_port: 7765
      metron_endpoint:
        host: foobar.com
      loggregator:
        tls:
          ca_cert: "((loggregator_ca.certificate))"
          doppler:
            cert: "((loggregator_tls_doppler.certificate))"
            key: "((loggregator_tls_doppler.private_key))"
- name: log-api
  azs:
  - z1
  - z2
  instances: 2
  vm_type: minimal
  stemcell: default
  update:
    serial: true
  networks:
  - name: default
  jobs:
  - name: loggregator_trafficcontroller
    release: loggregator
    consumes:
      doppler: {from: doppler}
    properties:
      uaa:
        internal_url: https://uaa.service.cf.internal:8443
        ca_cert: "((uaa_ca.certificate))"
      doppler:
        grpc_port: 6060
      loggregator:
        tls:
          cc_trafficcontroller:
            cert: "((loggregator_tls_cc_tc.certificate))"
            key: "((loggregator_tls_cc_tc.private_key))"
          ca_cert: "((loggregator_ca.certificate))"
          trafficcontroller:
            cert: "((loggregator_tls_tc.certificate))"
            key: "((loggregator_tls_tc.private_key))"
        uaa:
          client_secret: "((uaa_clients_doppler_secret))"
      system_domain: "((system_domain))"
      ssl:
        skip_cert_verify: true
      cc:
        internal_service_hostname: "cloud-controller-ng.service.cf.internal"
        tls_port: 9023
        mutual_tls:
          ca_cert: "((service_cf_internal_ca.certificate))"
releases:
- name: loggregator
  url: https://bosh.io/d/github.com/cloudfoundry/loggregator-release?v=105.0
  version: "105.0"
  sha1: d0bed91335aaac418eb6e8b2be13c6ecf4ce7b90
stemcells:
- alias: default
  os: ubuntu-xenial
  version: "250.17"
`
	err := yaml.Unmarshal([]byte(source), m)
	if err != nil {
		panic(err)
	}
	return m
}

// DefaultBOSHManifestConfigMap for tests
func (c *Catalog) DefaultBOSHManifestConfigMap(name string) corev1.ConfigMap {
	return corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Data: map[string]string{
			"manifest": `---
name: my-manifest
releases:
- name: nats
  version: "26"
  url: docker.io/cfcontainerization
  stemcell:
    os: opensuse-42.3
    version: 30.g9c91e77-30.80-7.0.0_257.gb97ced55
instance_groups:
- name: nats
  instances: 2
  jobs:
  - name: nats
    release: nats
    properties:
      nats:
        user: admin
        password: changeme
        debug: true
      bosh_containerization:
        ports:
        - name: "nats"
          protocol: "TCP"
          internal: 4222
        - name: "nats-routes"
          protocol: "TCP"
          internal: 4223
`,
		},
	}
}

// DefaultSecret for tests
func (c *Catalog) DefaultSecret(name string) corev1.Secret {
	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		StringData: map[string]string{
			name: "default-value",
		},
	}
}

// DefaultConfigMap for tests
func (c *Catalog) DefaultConfigMap(name string) corev1.ConfigMap {
	return corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Data: map[string]string{
			name: "default-value",
		},
	}
}

// DefaultBOSHDeployment fissile deployment CR
func (c *Catalog) DefaultBOSHDeployment(name, manifestRef string) bdcv1.BOSHDeployment {
	return bdcv1.BOSHDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: bdcv1.BOSHDeploymentSpec{
			Manifest: bdcv1.Manifest{Ref: manifestRef, Type: bdcv1.ConfigMapType},
		},
	}
}

// InterpolateOpsConfigMap for ops interpolate configmap tests
func (c *Catalog) InterpolateOpsConfigMap(name string) corev1.ConfigMap {
	return corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Data: map[string]string{
			"ops": `- type: replace
  path: /instance_groups/name=nats?/instances
  value: 5
`,
		},
	}
}

// InterpolateOpsSecret for ops interpolate secret tests
func (c *Catalog) InterpolateOpsSecret(name string) corev1.Secret {
	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		StringData: map[string]string{
			"ops": `- type: replace
  path: /instance_groups/name=nats?/instances
  value: 4
`,
		},
	}
}

// InterpolateOpsIncorrectSecret for ops interpolate incorrect secret tests
func (c *Catalog) InterpolateOpsIncorrectSecret(name string) corev1.Secret {
	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		StringData: map[string]string{
			"ops": `- type: remove
  path: /instance_groups/name=api
`,
		},
	}
}

// DefaultExtendedSecret for use in tests
func (c *Catalog) DefaultExtendedSecret(name string) esv1.ExtendedSecret {
	return esv1.ExtendedSecret{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: esv1.ExtendedSecretSpec{
			Type:       "password",
			SecretName: "generated-secret",
		},
	}
}

// DefaultCA for use in tests
func (c *Catalog) DefaultCA(name string, ca credsgen.Certificate) corev1.Secret {
	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Data: map[string][]byte{
			"ca":     ca.Certificate,
			"ca_key": ca.PrivateKey,
		},
	}
}

// DefaultExtendedStatefulSet for use in tests
func (c *Catalog) DefaultExtendedStatefulSet(name string) essv1.ExtendedStatefulSet {
	return essv1.ExtendedStatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: essv1.ExtendedStatefulSetSpec{
			Template: c.DefaultStatefulSet(name),
		},
	}
}

// WrongExtendedStatefulSet for use in tests
func (c *Catalog) WrongExtendedStatefulSet(name string) essv1.ExtendedStatefulSet {
	return essv1.ExtendedStatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: essv1.ExtendedStatefulSetSpec{
			Template: c.WrongStatefulSet(name),
		},
	}
}

// ExtendedStatefulSetWithPVC for use in tests
func (c *Catalog) ExtendedStatefulSetWithPVC(name, pvcName string) essv1.ExtendedStatefulSet {
	return essv1.ExtendedStatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: essv1.ExtendedStatefulSetSpec{
			Template: c.StatefulSetWithPVC(name, pvcName),
		},
	}
}

// WrongExtendedStatefulSetWithPVC for use in tests
func (c *Catalog) WrongExtendedStatefulSetWithPVC(name, pvcName string) essv1.ExtendedStatefulSet {
	return essv1.ExtendedStatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: essv1.ExtendedStatefulSetSpec{
			Template: c.WrongStatefulSetWithPVC(name, pvcName),
		},
	}
}

// OwnedReferencesExtendedStatefulSet for use in tests
func (c *Catalog) OwnedReferencesExtendedStatefulSet(name string) essv1.ExtendedStatefulSet {
	return essv1.ExtendedStatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: essv1.ExtendedStatefulSetSpec{
			UpdateOnConfigChange: true,
			Template:             c.OwnedReferencesStatefulSet(name),
		},
	}
}

// DefaultStatefulSet for use in tests
func (c *Catalog) DefaultStatefulSet(name string) v1beta2.StatefulSet {
	return v1beta2.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1beta2.StatefulSetSpec{
			Replicas: util.Int32(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"testpod": "yes",
				},
			},
			ServiceName: name,
			Template:    c.DefaultPodTemplate(name),
		},
	}
}

// StatefulSetWithPVC for use in tests
func (c *Catalog) StatefulSetWithPVC(name, pvcName string) v1beta2.StatefulSet {
	labels := map[string]string{
		"test-run-reference": name,
		"testpod":            "yes",
	}

	return v1beta2.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1beta2.StatefulSetSpec{
			Replicas: util.Int32(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			ServiceName:          name,
			Template:             c.PodTemplateWithLabelsAndMount(name, labels, pvcName),
			VolumeClaimTemplates: c.DefaultVolumeClaimTemplates(pvcName),
		},
	}
}

// WrongStatefulSetWithPVC for use in tests
func (c *Catalog) WrongStatefulSetWithPVC(name, pvcName string) v1beta2.StatefulSet {
	labels := map[string]string{
		"wrongpod":           "yes",
		"test-run-reference": name,
	}

	return v1beta2.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1beta2.StatefulSetSpec{
			Replicas: util.Int32(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			ServiceName:          name,
			Template:             c.WrongPodTemplateWithLabelsAndMount(name, labels, pvcName),
			VolumeClaimTemplates: c.DefaultVolumeClaimTemplates(pvcName),
		},
	}
}

// DefaultVolumeClaimTemplates for use in tests
func (c *Catalog) DefaultVolumeClaimTemplates(name string) []corev1.PersistentVolumeClaim {
	var storageClassName *string

	if class, ok := os.LookupEnv("OPERATOR_TEST_STORAGE_CLASS"); ok {
		storageClassName = &class
	}

	return []corev1.PersistentVolumeClaim{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				StorageClassName: storageClassName,
				AccessModes: []corev1.PersistentVolumeAccessMode{
					"ReadWriteOnce",
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceName(corev1.ResourceStorage): resource.MustParse("1G"),
					},
				},
			},
		},
	}
}

// DefaultStorageClass for use in tests
func (c *Catalog) DefaultStorageClass(name string) storagev1.StorageClass {
	reclaimPolicy := corev1.PersistentVolumeReclaimDelete
	volumeBindingMode := storagev1.VolumeBindingImmediate
	return storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Parameters: map[string]string{
			"path": "/tmp",
		},
		Provisioner:       "kubernetes.io/host-path",
		ReclaimPolicy:     &reclaimPolicy,
		VolumeBindingMode: &volumeBindingMode,
	}
}

// DefaultVolumeMount for use in tests
func (c *Catalog) DefaultVolumeMount(name string) corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      name,
		MountPath: "/etc/random",
	}
}

// WrongStatefulSet for use in tests
func (c *Catalog) WrongStatefulSet(name string) v1beta2.StatefulSet {
	return v1beta2.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1beta2.StatefulSetSpec{
			Replicas: util.Int32(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"wrongpod": "yes",
				},
			},
			ServiceName: name,
			Template:    c.WrongPodTemplate(name),
		},
	}
}

// OwnedReferencesStatefulSet for use in tests
func (c *Catalog) OwnedReferencesStatefulSet(name string) v1beta2.StatefulSet {
	return v1beta2.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1beta2.StatefulSetSpec{
			Replicas: util.Int32(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"referencedpod": "yes",
				},
			},
			ServiceName: name,
			Template:    c.OwnedReferencesPodTemplate(name),
		},
	}
}

// PodTemplateWithLabelsAndMount defines a pod template with a simple web server useful for testing
func (c *Catalog) PodTemplateWithLabelsAndMount(name string, labels map[string]string, pvcName string) corev1.PodTemplateSpec {
	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Spec: corev1.PodSpec{
			TerminationGracePeriodSeconds: util.Int64(1),
			Containers: []corev1.Container{
				{
					Name:    "busybox",
					Image:   "busybox",
					Command: []string{"sleep", "3600"},
					VolumeMounts: []corev1.VolumeMount{
						c.DefaultVolumeMount(pvcName),
					},
				},
			},
		},
	}
}

// WrongPodTemplateWithLabelsAndMount defines a pod template with a simple web server useful for testing
func (c *Catalog) WrongPodTemplateWithLabelsAndMount(name string, labels map[string]string, pvcName string) corev1.PodTemplateSpec {
	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Spec: corev1.PodSpec{
			TerminationGracePeriodSeconds: util.Int64(1),
			Containers: []corev1.Container{
				{
					Name:  "wrong-container",
					Image: "wrong-image",
					VolumeMounts: []corev1.VolumeMount{
						c.DefaultVolumeMount(pvcName),
					},
				},
			},
		},
	}
}

// DefaultPodTemplate defines a pod template with a simple web server useful for testing
func (c *Catalog) DefaultPodTemplate(name string) corev1.PodTemplateSpec {
	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"testpod": "yes",
			},
		},
		Spec: c.Sleep1hPodSpec(),
	}
}

// WrongPodTemplate defines a pod template with a simple web server useful for testing
func (c *Catalog) WrongPodTemplate(name string) corev1.PodTemplateSpec {
	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"wrongpod": "yes",
			},
		},
		Spec: corev1.PodSpec{
			TerminationGracePeriodSeconds: util.Int64(1),
			Containers: []corev1.Container{
				{
					Name:  "wrong-container",
					Image: "wrong-image",
				},
			},
		},
	}
}

// OwnedReferencesPodTemplate defines a pod template with four references from VolumeSources, EnvFrom and Env
func (c *Catalog) OwnedReferencesPodTemplate(name string) corev1.PodTemplateSpec {
	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"referencedpod": "yes",
			},
		},
		Spec: corev1.PodSpec{
			TerminationGracePeriodSeconds: util.Int64(1),
			Volumes: []corev1.Volume{
				{
					Name: "secret1",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "example1",
						},
					},
				},
				{
					Name: "configmap1",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "example1",
							},
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:    "container1",
					Image:   "busybox",
					Command: []string{"sleep", "3600"},
					EnvFrom: []corev1.EnvFromSource{
						{
							ConfigMapRef: &corev1.ConfigMapEnvSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "example1",
								},
							},
						},
						{
							SecretRef: &corev1.SecretEnvSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "example1",
								},
							},
						},
					},
				},
				{
					Name:    "container2",
					Image:   "busybox",
					Command: []string{"sleep", "3600"},
					Env: []corev1.EnvVar{
						{
							Name: "ENV1",
							ValueFrom: &corev1.EnvVarSource{
								ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
									Key: "example2",
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "example2",
									},
								},
							},
						},
						{
							Name: "ENV2",
							ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									Key: "example2",
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "example2",
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// CmdPodTemplate returns the spec with a given command for busybox
func (c *Catalog) CmdPodTemplate(cmd []string) corev1.PodTemplateSpec {
	return corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			RestartPolicy:                 corev1.RestartPolicyNever,
			TerminationGracePeriodSeconds: util.Int64(1),
			Containers: []corev1.Container{
				{
					Name:    "busybox",
					Image:   "busybox",
					Command: cmd,
				},
			},
		},
	}
}

// ConfigPodTemplate returns the spec with a given command for busybox
func (c *Catalog) ConfigPodTemplate() corev1.PodTemplateSpec {
	one := int64(1)
	return corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			RestartPolicy:                 corev1.RestartPolicyNever,
			TerminationGracePeriodSeconds: &one,
			Volumes: []corev1.Volume{
				{
					Name: "secret1",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "secret1",
						},
					},
				},
				{
					Name: "configmap1",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "config1",
							},
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:    "busybox",
					Image:   "busybox",
					Command: []string{"sleep", "1"},
				},
			},
		},
	}
}

// MultiContainerPodTemplate returns the spec with two containers running a given command for busybox
func (c *Catalog) MultiContainerPodTemplate(cmd []string) corev1.PodTemplateSpec {
	return corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			RestartPolicy:                 corev1.RestartPolicyNever,
			TerminationGracePeriodSeconds: util.Int64(1),
			Containers: []corev1.Container{
				{
					Name:    "busybox",
					Image:   "busybox",
					Command: cmd,
				},
				{
					Name:    "busybox2",
					Image:   "busybox",
					Command: cmd,
				},
			},
		},
	}
}

// FailingMultiContainerPodTemplate returns a spec with a given command for busybox and a second container which fails
func (c *Catalog) FailingMultiContainerPodTemplate(cmd []string) corev1.PodTemplateSpec {
	return corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			RestartPolicy:                 corev1.RestartPolicyNever,
			TerminationGracePeriodSeconds: util.Int64(1),
			Containers: []corev1.Container{
				{
					Name:    "busybox",
					Image:   "busybox",
					Command: cmd,
				},
				{
					Name:    "failing",
					Image:   "busybox",
					Command: []string{"exit", "1"},
				},
			},
		},
	}
}

// DefaultPod defines a pod with a simple web server useful for testing
func (c *Catalog) DefaultPod(name string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: c.Sleep1hPodSpec(),
	}
}

// LabeledPod defines a pod with labels and a simple web server
func (c *Catalog) LabeledPod(name string, labels map[string]string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Spec: c.Sleep1hPodSpec(),
	}
}

// AnnotatedPod defines a pod with annotations
func (c *Catalog) AnnotatedPod(name string, annotations map[string]string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Annotations: annotations,
		},
		Spec: c.Sleep1hPodSpec(),
	}
}

// Sleep1hPodSpec defines a simple pod that sleeps 60*60s for testing
func (c *Catalog) Sleep1hPodSpec() corev1.PodSpec {
	return corev1.PodSpec{
		TerminationGracePeriodSeconds: util.Int64(1),
		Containers: []corev1.Container{
			{
				Name:    "busybox",
				Image:   "busybox",
				Command: []string{"sleep", "3600"},
			},
		},
	}
}

// EmptyBOSHDeployment empty fissile deployment CR
func (c *Catalog) EmptyBOSHDeployment(name, manifestRef string) bdcv1.BOSHDeployment {
	return bdcv1.BOSHDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       bdcv1.BOSHDeploymentSpec{},
	}
}

// DefaultBOSHDeploymentWithOps fissile deployment CR with ops
func (c *Catalog) DefaultBOSHDeploymentWithOps(name, manifestRef string, opsRef string) bdcv1.BOSHDeployment {
	return bdcv1.BOSHDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: bdcv1.BOSHDeploymentSpec{
			Manifest: bdcv1.Manifest{Ref: manifestRef, Type: bdcv1.ConfigMapType},
			Ops: []bdcv1.Ops{
				{Ref: opsRef, Type: bdcv1.ConfigMapType},
			},
		},
	}
}

// WrongTypeBOSHDeployment fissile deployment CR containing wrong type
func (c *Catalog) WrongTypeBOSHDeployment(name, manifestRef string) bdcv1.BOSHDeployment {
	return bdcv1.BOSHDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: bdcv1.BOSHDeploymentSpec{
			Manifest: bdcv1.Manifest{Ref: manifestRef, Type: "wrong-type"},
		},
	}
}

// BOSHDeploymentWithWrongTypeOps fissile deployment CR with wrong type ops
func (c *Catalog) BOSHDeploymentWithWrongTypeOps(name, manifestRef string, opsRef string) bdcv1.BOSHDeployment {
	return bdcv1.BOSHDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: bdcv1.BOSHDeploymentSpec{
			Manifest: bdcv1.Manifest{Ref: manifestRef, Type: bdcv1.ConfigMapType},
			Ops: []bdcv1.Ops{
				{Ref: opsRef, Type: "wrong-type"},
			},
		},
	}
}

// InterpolateBOSHDeployment fissile deployment CR
func (c *Catalog) InterpolateBOSHDeployment(name, manifestRef, opsRef string, secretRef string) bdcv1.BOSHDeployment {
	return bdcv1.BOSHDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: bdcv1.BOSHDeploymentSpec{
			Manifest: bdcv1.Manifest{Ref: manifestRef, Type: bdcv1.ConfigMapType},
			Ops: []bdcv1.Ops{
				{Ref: opsRef, Type: bdcv1.ConfigMapType},
				{Ref: secretRef, Type: bdcv1.SecretType},
			},
		},
	}
}

// LabelTriggeredExtendedJob allows customization of labels triggers
func (c *Catalog) LabelTriggeredExtendedJob(name string, state ejv1.PodState, ml labels.Set, me []*ejv1.Requirement, cmd []string) *ejv1.ExtendedJob {
	return &ejv1.ExtendedJob{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: ejv1.ExtendedJobSpec{
			Trigger: ejv1.Trigger{
				Strategy: "podstate",
				PodState: &ejv1.PodStateTrigger{
					When: state,
					Selector: &ejv1.Selector{
						MatchLabels:      &ml,
						MatchExpressions: me,
					},
				},
			},
			Template: c.CmdPodTemplate(cmd),
		},
	}
}

// DefaultExtendedJob default values
func (c *Catalog) DefaultExtendedJob(name string) *ejv1.ExtendedJob {
	return c.LabelTriggeredExtendedJob(
		name,
		ejv1.PodStateReady,
		map[string]string{"key": "value"},
		[]*ejv1.Requirement{},
		[]string{"sleep", "1"},
	)
}

// LongRunningExtendedJob has a longer sleep time
func (c *Catalog) LongRunningExtendedJob(name string) *ejv1.ExtendedJob {
	return c.LabelTriggeredExtendedJob(
		name,
		ejv1.PodStateReady,
		map[string]string{"key": "value"},
		[]*ejv1.Requirement{},
		[]string{"sleep", "15"},
	)
}

// OnDeleteExtendedJob runs for deleted pods
func (c *Catalog) OnDeleteExtendedJob(name string) *ejv1.ExtendedJob {
	return c.LabelTriggeredExtendedJob(
		name,
		ejv1.PodStateDeleted,
		map[string]string{"key": "value"},
		[]*ejv1.Requirement{},
		[]string{"sleep", "1"},
	)
}

// MatchExpressionExtendedJob uses Matchexpressions for matching
func (c *Catalog) MatchExpressionExtendedJob(name string) *ejv1.ExtendedJob {
	return c.LabelTriggeredExtendedJob(
		name,
		ejv1.PodStateReady,
		map[string]string{},
		[]*ejv1.Requirement{
			{Key: "env", Operator: "in", Values: []string{"production"}},
		},
		[]string{"sleep", "1"},
	)
}

// ComplexMatchExtendedJob uses MatchLabels and MatchExpressions
func (c *Catalog) ComplexMatchExtendedJob(name string) *ejv1.ExtendedJob {
	return c.LabelTriggeredExtendedJob(
		name,
		ejv1.PodStateReady,
		map[string]string{"key": "value"},
		[]*ejv1.Requirement{
			{Key: "env", Operator: "in", Values: []string{"production"}},
		},
		[]string{"sleep", "1"},
	)
}

// OutputExtendedJob persists its output
func (c *Catalog) OutputExtendedJob(name string, template corev1.PodTemplateSpec) *ejv1.ExtendedJob {
	return &ejv1.ExtendedJob{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: ejv1.ExtendedJobSpec{
			Trigger: ejv1.Trigger{
				Strategy: "podstate",
				PodState: &ejv1.PodStateTrigger{
					When:     "ready",
					Selector: &ejv1.Selector{MatchLabels: &labels.Set{"key": "value"}},
				},
			},
			Template: template,
			Output: &ejv1.Output{
				NamePrefix:   name + "-output-",
				OutputType:   "json",
				SecretLabels: map[string]string{"label-key": "label-value", "label-key2": "label-value2"},
			},
		},
	}
}

// DefaultExtendedJobWithSucceededJob returns an ExtendedJob and a Job owned by it
func (c *Catalog) DefaultExtendedJobWithSucceededJob(name string) (*ejv1.ExtendedJob, *batchv1.Job, *corev1.Pod) {
	ejob := c.DefaultExtendedJob(name)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: name + "-job",
			OwnerReferences: []metav1.OwnerReference{
				{
					Name:       name,
					UID:        "",
					Controller: util.Bool(true),
				},
			},
		},
		Status: batchv1.JobStatus{Succeeded: 1},
	}
	pod := c.DefaultPod(name + "-pod")
	pod.Labels = map[string]string{
		"job-name": job.GetName(),
	}
	return ejob, job, &pod
}

// ErrandExtendedJob default values
func (c *Catalog) ErrandExtendedJob(name string) ejv1.ExtendedJob {
	cmd := []string{"sleep", "1"}
	return ejv1.ExtendedJob{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: ejv1.ExtendedJobSpec{
			Trigger: ejv1.Trigger{
				Strategy: ejv1.TriggerNow,
			},
			Template: c.CmdPodTemplate(cmd),
		},
	}
}

// AutoErrandExtendedJob default values
func (c *Catalog) AutoErrandExtendedJob(name string) ejv1.ExtendedJob {
	cmd := []string{"sleep", "1"}
	return ejv1.ExtendedJob{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: ejv1.ExtendedJobSpec{
			Trigger: ejv1.Trigger{
				Strategy: ejv1.TriggerOnce,
			},
			Template: c.CmdPodTemplate(cmd),
		},
	}
}
