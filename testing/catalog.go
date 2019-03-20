// Package testing contains methods to create test data. It's a seaparate
// package to avoid import cycles. Helper functions can be found in the package
// `testhelper`.
package testing

import (
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
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/context"
	helper "code.cloudfoundry.org/cf-operator/pkg/testhelper"
	"k8s.io/apimachinery/pkg/labels"
)

// Catalog provides several instances for tests
type Catalog struct{}

// DefaultContextConfig for tests
func (c *Catalog) DefaultContextConfig() *context.Config {
	return &context.Config{ //Set the context to be TODO
		CtxTimeOut:        10 * time.Second,
		CtxType:           context.NewContext(),
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
	yaml.Unmarshal([]byte(source), &m)
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
- name: fissile-nats
  version: "26"
  url: docker.io/cfcontainerization
  stemcell:
    os: opensuse-42.3
    version: 28.g837c5b3-30.79-7.0.0_237.g8a9ed8f
instance_groups:
- name: nats
  instances: 1
  jobs:
  - name: nats
    release: fissile-nats
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
func (c *Catalog) ExtendedStatefulSetWithPVC(name, pvcName, storageClass string) essv1.ExtendedStatefulSet {
	return essv1.ExtendedStatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: essv1.ExtendedStatefulSetSpec{
			Template: c.StatefulSetWithPVC(name, pvcName, storageClass),
		},
	}
}

// WrongExtendedStatefulSetWithPVC for use in tests
func (c *Catalog) WrongExtendedStatefulSetWithPVC(name, pvcName, storageClass string) essv1.ExtendedStatefulSet {
	return essv1.ExtendedStatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: essv1.ExtendedStatefulSetSpec{
			Template: c.WrongStatefulSetWithPVC(name, pvcName, storageClass),
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
			UpdateOnEnvChange: true,
			Template:          c.OwnedReferencesStatefulSet(name),
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
			Replicas: helper.Int32(1),
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
func (c *Catalog) StatefulSetWithPVC(name, pvcName, storageClass string) v1beta2.StatefulSet {
	labels := map[string]string{
		"test-run-reference": name,
		"testpod":            "yes",
	}

	return v1beta2.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1beta2.StatefulSetSpec{
			Replicas: helper.Int32(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			ServiceName:          name,
			Template:             c.PodTemplateWithLabelsAndMount(name, labels, pvcName),
			VolumeClaimTemplates: c.DefaultVolumeClaimTemplates(pvcName, storageClass),
		},
	}
}

// WrongStatefulSetWithPVC for use in tests
func (c *Catalog) WrongStatefulSetWithPVC(name, pvcName, storageClass string) v1beta2.StatefulSet {
	labels := map[string]string{
		"wrongpod":           "yes",
		"test-run-reference": name,
	}

	return v1beta2.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1beta2.StatefulSetSpec{
			Replicas: helper.Int32(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			ServiceName:          name,
			Template:             c.WrongPodTemplateWithLabelsAndMount(name, labels, pvcName),
			VolumeClaimTemplates: c.DefaultVolumeClaimTemplates(pvcName, storageClass),
		},
	}
}

// DefaultVolumeClaimTemplates for use in tests
func (c *Catalog) DefaultVolumeClaimTemplates(name, storageClass string) []corev1.PersistentVolumeClaim {
	return []corev1.PersistentVolumeClaim{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				StorageClassName: &storageClass,
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
			Replicas: helper.Int32(1),
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
			Replicas: helper.Int32(1),
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
			TerminationGracePeriodSeconds: helper.Int64(1),
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
			TerminationGracePeriodSeconds: helper.Int64(1),
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
			TerminationGracePeriodSeconds: helper.Int64(1),
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
			TerminationGracePeriodSeconds: helper.Int64(1),
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
			TerminationGracePeriodSeconds: helper.Int64(1),
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

// MultiContainerPodTemplate returns the spec with two containers running a given command for busybox
func (c *Catalog) MultiContainerPodTemplate(cmd []string) corev1.PodTemplateSpec {
	return corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			RestartPolicy:                 corev1.RestartPolicyNever,
			TerminationGracePeriodSeconds: helper.Int64(1),
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
			TerminationGracePeriodSeconds: helper.Int64(1),
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
		TerminationGracePeriodSeconds: helper.Int64(1),
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
					Controller: helper.Bool(true),
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
