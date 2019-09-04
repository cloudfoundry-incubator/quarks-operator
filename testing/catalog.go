// Package testing contains methods to create test data. It's a seaparate
// package to avoid import cycles. Helper functions can be found in the package
// `testhelper`.
package testing

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"k8s.io/api/apps/v1beta2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/cf-operator/pkg/credsgen"
	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	esv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedsecret/v1alpha1"
	essv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	bm "code.cloudfoundry.org/cf-operator/testing/boshmanifest"
)

const (
	manifestFailedMessage = "Loading bosh manifest spec failed."
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
func (c *Catalog) DefaultBOSHManifest() (*manifest.Manifest, error) {
	m, err := manifest.LoadYAML([]byte(bm.Default))
	if err != nil {
		return &manifest.Manifest{}, errors.Wrapf(err, "Loading default manifest spec failed.")
	}
	return m, nil
}

// ElaboratedBOSHManifest for data gathering tests
func (c *Catalog) ElaboratedBOSHManifest() (*manifest.Manifest, error) {
	m, err := manifest.LoadYAML([]byte(bm.Elaborated))
	if err != nil {
		return &manifest.Manifest{}, errors.Wrapf(err, manifestFailedMessage)
	}
	return m, nil
}

// BOSHManifestWithProviderAndConsumer for data gathering tests
func (c *Catalog) BOSHManifestWithProviderAndConsumer() (*manifest.Manifest, error) {
	m, err := manifest.LoadYAML([]byte(bm.WithProviderAndConsumer))
	if err != nil {
		return &manifest.Manifest{}, errors.Wrapf(err, manifestFailedMessage)
	}
	return m, nil
}

// BOSHManifestWithOverriddenBPMInfo for data gathering tests
func (c *Catalog) BOSHManifestWithOverriddenBPMInfo() (*manifest.Manifest, error) {
	m, err := manifest.LoadYAML([]byte(bm.WithOverriddenBPMInfo))
	if err != nil {
		return &manifest.Manifest{}, errors.Wrapf(err, manifestFailedMessage)
	}
	return m, nil
}

// BOSHManifestWithAbsentBPMInfo for data gathering tests
func (c *Catalog) BOSHManifestWithAbsentBPMInfo() (*manifest.Manifest, error) {
	m, err := manifest.LoadYAML([]byte(bm.WithAbsentBPMInfo))
	if err != nil {
		return &manifest.Manifest{}, errors.Wrapf(err, manifestFailedMessage)
	}
	return m, nil
}

// BOSHManifestWithMultiBPMProcesses returns a manifest with multi BPM configuration
func (c *Catalog) BOSHManifestWithMultiBPMProcesses() (*manifest.Manifest, error) {
	m, err := manifest.LoadYAML([]byte(bm.WithMultiBPMProcesses))
	if err != nil {
		return &manifest.Manifest{}, errors.Wrapf(err, manifestFailedMessage)
	}
	return m, nil
}

// BOSHManifestWithMultiBPMProcessesAndPersistentDisk returns a manifest with multi BPM configuration and persistent disk
func (c *Catalog) BOSHManifestWithMultiBPMProcessesAndPersistentDisk() (*manifest.Manifest, error) {
	m, err := manifest.LoadYAML([]byte(bm.WithMultiBPMProcessesAndPersistentDisk))
	if err != nil {
		return &manifest.Manifest{}, errors.Wrapf(err, manifestFailedMessage)
	}
	return m, nil
}

// BOSHManifestCFRouting returns a manifest for the CF routing release with an underscore in the name
func (c *Catalog) BOSHManifestCFRouting() (*manifest.Manifest, error) {
	m, err := manifest.LoadYAML([]byte(bm.CFRouting))
	if err != nil {
		return &manifest.Manifest{}, errors.Wrapf(err, manifestFailedMessage)
	}
	return m, nil
}

// BOSHManifestWithBPMRelease returns a manifest with single BPM configuration
func (c *Catalog) BOSHManifestWithBPMRelease() (*manifest.Manifest, error) {
	m, err := manifest.LoadYAML([]byte(bm.BPMRelease))
	if err != nil {
		return &manifest.Manifest{}, errors.Wrapf(err, manifestFailedMessage)
	}
	return m, nil
}

// BOSHManifestWithoutPersistentDisk returns a manifest with persistent disk declaration
func (c *Catalog) BOSHManifestWithoutPersistentDisk() (*manifest.Manifest, error) {
	m, err := manifest.LoadYAML([]byte(bm.BPMReleaseWithoutPersistentDisk))
	if err != nil {
		return &manifest.Manifest{}, errors.Wrapf(err, manifestFailedMessage)
	}
	return m, nil
}

// BPMReleaseWithAffinity returns a manifest with affinity
func (c *Catalog) BPMReleaseWithAffinity() (*manifest.Manifest, error) {
	m, err := manifest.LoadYAML([]byte(bm.BPMReleaseWithAffinity))
	if err != nil {
		return &manifest.Manifest{}, errors.Wrapf(err, manifestFailedMessage)
	}
	return m, nil
}

// BPMReleaseWithAffinityConfigMap for tests
func (c *Catalog) BPMReleaseWithAffinityConfigMap(name string) corev1.ConfigMap {
	return corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Data: map[string]string{
			"manifest": bm.BPMReleaseWithAffinity,
		},
	}
}

// DefaultBOSHManifestConfigMap for tests
func (c *Catalog) DefaultBOSHManifestConfigMap(name string) corev1.ConfigMap {
	return corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Data: map[string]string{
			"manifest": bm.NatsSmall,
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

// StorageClassSecret for tests
func (c *Catalog) StorageClassSecret(name string, class string) corev1.Secret {
	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		StringData: map[string]string{
			"value": class,
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

// DefaultBOSHDeployment a deployment CR
func (c *Catalog) DefaultBOSHDeployment(name, manifestRef string) bdv1.BOSHDeployment {
	return bdv1.BOSHDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: bdv1.BOSHDeploymentSpec{
			Manifest: bdv1.ResourceReference{Name: manifestRef, Type: bdv1.ConfigMapReference},
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
  value: 1
`,
		},
	}
}

// BOSHManifestConfigMapWithTwoInstanceGroups for tests
func (c *Catalog) BOSHManifestConfigMapWithTwoInstanceGroups(name string) corev1.ConfigMap {
	return corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Data: map[string]string{
			"manifest": bm.BOSHManifestWithTwoInstanceGroups,
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
  value: 3
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
func (c *Catalog) ExtendedStatefulSetWithPVC(name, pvcName string, storageClassName string) essv1.ExtendedStatefulSet {
	return essv1.ExtendedStatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: essv1.ExtendedStatefulSetSpec{
			Template: c.StatefulSetWithPVC(name, pvcName, storageClassName),
		},
	}
}

// WrongExtendedStatefulSetWithPVC for use in tests
func (c *Catalog) WrongExtendedStatefulSetWithPVC(name, pvcName string, storageClassName string) essv1.ExtendedStatefulSet {
	return essv1.ExtendedStatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: essv1.ExtendedStatefulSetSpec{
			Template: c.WrongStatefulSetWithPVC(name, pvcName, storageClassName),
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
func (c *Catalog) StatefulSetWithPVC(name, pvcName string, storageClassName string) v1beta2.StatefulSet {
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
			VolumeClaimTemplates: c.DefaultVolumeClaimTemplates(pvcName, storageClassName),
		},
	}
}

// WrongStatefulSetWithPVC for use in tests
func (c *Catalog) WrongStatefulSetWithPVC(name, pvcName string, storageClassName string) v1beta2.StatefulSet {
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
			VolumeClaimTemplates: c.DefaultVolumeClaimTemplates(pvcName, storageClassName),
		},
	}
}

// DefaultVolumeClaimTemplates for use in tests
func (c *Catalog) DefaultVolumeClaimTemplates(name string, storageClassName string) []corev1.PersistentVolumeClaim {

	return []corev1.PersistentVolumeClaim{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				StorageClassName: &storageClassName,
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
					Env: []corev1.EnvVar{
						{Name: "REPLICAS", Value: "1"},
						{Name: "AZ_INDEX", Value: "1"},
						{Name: "POD_ORDINAL", Value: "0"},
					},
				},
			},
		},
	}
}

// ConfigPodTemplate returns the spec with a given command for busybox
func (c *Catalog) ConfigPodTemplate() corev1.PodTemplateSpec {
	one := int64(1)
	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{"delete": "pod"},
		},
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
					Env: []corev1.EnvVar{
						{Name: "REPLICAS", Value: "1"},
						{Name: "AZ_INDEX", Value: "1"},
						{Name: "POD_ORDINAL", Value: "0"},
					},
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

// EmptyBOSHDeployment empty deployment CR
func (c *Catalog) EmptyBOSHDeployment(name, manifestRef string) bdv1.BOSHDeployment {
	return bdv1.BOSHDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       bdv1.BOSHDeploymentSpec{},
	}
}

// DefaultBOSHDeploymentWithOps a deployment CR with ops
func (c *Catalog) DefaultBOSHDeploymentWithOps(name, manifestRef string, opsRef string) bdv1.BOSHDeployment {
	return bdv1.BOSHDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: bdv1.BOSHDeploymentSpec{
			Manifest: bdv1.ResourceReference{Name: manifestRef, Type: bdv1.ConfigMapReference},
			Ops: []bdv1.ResourceReference{
				{Name: opsRef, Type: bdv1.ConfigMapReference},
			},
		},
	}
}

// WrongTypeBOSHDeployment a deployment CR containing wrong type
func (c *Catalog) WrongTypeBOSHDeployment(name, manifestRef string) bdv1.BOSHDeployment {
	return bdv1.BOSHDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: bdv1.BOSHDeploymentSpec{
			Manifest: bdv1.ResourceReference{Name: manifestRef, Type: "wrong-type"},
		},
	}
}

// BOSHDeploymentWithWrongTypeOps a deployment CR with wrong type ops
func (c *Catalog) BOSHDeploymentWithWrongTypeOps(name, manifestRef string, opsRef string) bdv1.BOSHDeployment {
	return bdv1.BOSHDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: bdv1.BOSHDeploymentSpec{
			Manifest: bdv1.ResourceReference{Name: manifestRef, Type: bdv1.ConfigMapReference},
			Ops: []bdv1.ResourceReference{
				{Name: opsRef, Type: "wrong-type"},
			},
		},
	}
}

// InterpolateBOSHDeployment a deployment CR
func (c *Catalog) InterpolateBOSHDeployment(name, manifestRef, opsRef string, secretRef string) bdv1.BOSHDeployment {
	return bdv1.BOSHDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: bdv1.BOSHDeploymentSpec{
			Manifest: bdv1.ResourceReference{Name: manifestRef, Type: bdv1.ConfigMapReference},
			Ops: []bdv1.ResourceReference{
				{Name: opsRef, Type: bdv1.ConfigMapReference},
				{Name: secretRef, Type: bdv1.SecretReference},
			},
		},
	}
}

// DefaultExtendedJob default values
func (c *Catalog) DefaultExtendedJob(name string) *ejv1.ExtendedJob {
	cmd := []string{"sleep", "1"}
	return &ejv1.ExtendedJob{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: ejv1.ExtendedJobSpec{
			Trigger: ejv1.Trigger{
				Strategy: ejv1.TriggerNow,
			},
			Template: c.CmdPodTemplate(cmd),
		},
	}
}

// DefaultExtendedJobWithSucceededJob returns an ExtendedJob and a Job owned by it
func (c *Catalog) DefaultExtendedJobWithSucceededJob(name string) (*ejv1.ExtendedJob, *batchv1.Job, *corev1.Pod) {
	ejob := c.DefaultExtendedJob(name)
	backoffLimit := util.Int32(2)
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
		Spec:   batchv1.JobSpec{BackoffLimit: backoffLimit},
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
