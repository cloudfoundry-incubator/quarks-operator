// Package testing contains methods to create test data. It's a seaparate
// package to avoid import cycles. Helper functions can be found in the package
// `testhelper`.
package testing

import (
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/afero"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/cf-operator/pkg/credsgen"
	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers/statefulset"
	bm "code.cloudfoundry.org/cf-operator/testing/boshmanifest"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/names"
	"code.cloudfoundry.org/quarks-utils/pkg/pointers"
)

const (
	manifestFailedMessage = "Loading bosh manifest spec failed."
)

// Catalog provides several instances for tests
type Catalog struct{}

// DefaultConfig for tests
func (c *Catalog) DefaultConfig() *config.Config {
	return &config.Config{
		CtxTimeOut:        10 * time.Second,
		OperatorNamespace: "default",
		Namespace:         "staging",
		WebhookServerHost: "foo.com",
		WebhookServerPort: 1234,
		Fs:                afero.NewMemMapFs(),
	}
}

// DefaultBOSHManifest returns a BOSH manifest for unit tests
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

// BOSHManifestWithResources for data gathering tests
func (c *Catalog) BOSHManifestWithResources() (*manifest.Manifest, error) {
	m, err := manifest.LoadYAML([]byte(bm.WithResources))
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

// BOSHManifestWithLinks returns a manifest with explicit and implicit BOSH links
// Also usable in integration tests
func (c *Catalog) BOSHManifestWithLinks() (*manifest.Manifest, error) {
	m, err := manifest.LoadYAML([]byte(bm.NatsSmallWithLinks))
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

// BOSHManifestWithZeroInstances for data gathering tests
func (c *Catalog) BOSHManifestWithZeroInstances() (*manifest.Manifest, error) {
	m, err := manifest.LoadYAML([]byte(bm.WithZeroInstances))
	if err != nil {
		return &manifest.Manifest{}, errors.Wrapf(err, manifestFailedMessage)
	}
	return m, nil
}

// BOSHManifestWithExternalLinks returns a manifest with external links
// Also usable in integration tests
func (c *Catalog) BOSHManifestWithExternalLinks() (*manifest.Manifest, error) {
	m, err := manifest.LoadYAML([]byte(bm.ManifestWithExternalLinks))
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

// DefaultBOSHManifestConfigMap for integration tests
func (c *Catalog) DefaultBOSHManifestConfigMap(name string) corev1.ConfigMap {
	return corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Data: map[string]string{
			"manifest": bm.NatsSmall,
		},
	}
}

// BOSHManifestSecret for tests
func (c *Catalog) BOSHManifestSecret(ref string, text string) corev1.Secret {
	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: ref},
		StringData: map[string]string{
			"manifest": text,
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

// QuarksLinkSecret returns a link secret, as generated for consumption by an external (non BOSH) consumer
func (c *Catalog) QuarksLinkSecret(deploymentName, igName, linkType, linkName, value string) corev1.Secret {
	key := names.EntanglementSecretKey(linkType, linkName)
	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "link-" + deploymentName + "-" + igName,
			Labels: map[string]string{
				manifest.LabelDeploymentName: deploymentName,
			},
		},
		Data: map[string][]byte{
			key: []byte(value),
		},
	}
}

// DefaultQuarksLinkSecret has default values from the nats release
func (c *Catalog) DefaultQuarksLinkSecret(deploymentName, linkType string) corev1.Secret {
	return c.QuarksLinkSecret(
		deploymentName, linkType, // link-<nats-deployment>-<nats-ig>
		linkType, "nats", // type.name
		`{"nats":{"password":"custom_password","port":4222,"user":"admin"}}`,
	)
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

// DefaultStatefulSet for use in tests
func (c *Catalog) DefaultStatefulSet(name string) appsv1.StatefulSet {
	return appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"testpod": "yes",
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: pointers.Int32(1),
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

// DefaultStatefulSetWithActiveSinglePod for use in tests
func (c *Catalog) DefaultStatefulSetWithActiveSinglePod(name string) appsv1.StatefulSet {
	return appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"testpod": "yes",
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: pointers.Int32(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"testpod": "yes",
				},
			},
			ServiceName: name,
			Template:    c.DefaultPodTemplateWithActiveLabel(name),
		},
	}
}

// DefaultStatefulSetWithReplicasN for use in tests
func (c *Catalog) DefaultStatefulSetWithReplicasN(name string) appsv1.StatefulSet {
	return appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"testpod": "yes",
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: pointers.Int32(3),
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
func (c *Catalog) StatefulSetWithPVC(name, pvcName string, storageClassName string) appsv1.StatefulSet {
	labels := map[string]string{
		"test-run-reference": name,
		"testpod":            "yes",
	}

	return appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: pointers.Int32(1),
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
func (c *Catalog) WrongStatefulSetWithPVC(name, pvcName string, storageClassName string) appsv1.StatefulSet {
	labels := map[string]string{
		"wrongpod":           "yes",
		"test-run-reference": name,
	}

	return appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: pointers.Int32(1),
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
func (c *Catalog) WrongStatefulSet(name string) appsv1.StatefulSet {
	return appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Annotations: map[string]string{
				statefulset.AnnotationCanaryWatchTime: "30000",
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: pointers.Int32(1),
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
func (c *Catalog) OwnedReferencesStatefulSet(name string) appsv1.StatefulSet {
	return appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: pointers.Int32(1),
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
			TerminationGracePeriodSeconds: pointers.Int64(1),
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
			TerminationGracePeriodSeconds: pointers.Int64(1),
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

// DefaultPodTemplateWithActiveLabel defines a pod template with a simple web server useful for testing
func (c *Catalog) DefaultPodTemplateWithActiveLabel(name string) corev1.PodTemplateSpec {
	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"testpod":                            "yes",
				"quarks.cloudfoundry.org/pod-active": "active",
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
			TerminationGracePeriodSeconds: pointers.Int64(1),
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
			TerminationGracePeriodSeconds: pointers.Int64(1),
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

// EntangledPod is a pod which has annotations for a BOSH link
func (c *Catalog) EntangledPod(deploymentName string) corev1.Pod {
	return c.AnnotatedPod(
		"entangled",
		map[string]string{
			"quarks.cloudfoundry.org/deployment": deploymentName,
			"quarks.cloudfoundry.org/consumes":   "nats.nats",
		},
	)
}

// Sleep1hPodSpec defines a simple pod that sleeps 60*60s for testing
func (c *Catalog) Sleep1hPodSpec() corev1.PodSpec {
	return corev1.PodSpec{
		TerminationGracePeriodSeconds: pointers.Int64(1),
		Containers: []corev1.Container{
			{
				Name:    "busybox",
				Image:   "busybox",
				Command: []string{"sleep", "3600"},
			},
		},
	}
}

// NodePortService returns a Service of type NodePort
func (c *Catalog) NodePortService(name, ig string, targetPort int32) corev1.Service {
	return corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeNodePort,
			Selector: map[string]string{
				names.GroupName + "/instance-group-name": ig,
			},
			Ports: []corev1.ServicePort{
				corev1.ServicePort{
					Port:       targetPort,
					TargetPort: intstr.FromInt(int(targetPort)),
				},
			},
		},
	}
}

//BOSHManifestWithGlobalUpdateBlock returns a manifest with a global update block
func (c *Catalog) BOSHManifestWithGlobalUpdateBlock() (*manifest.Manifest, error) {
	m, err := manifest.LoadYAML([]byte(bm.BPMReleaseWithGlobalUpdateBlock))
	if err != nil {
		return &manifest.Manifest{}, errors.Wrapf(err, manifestFailedMessage)
	}
	return m, nil
}

// BOSHManifestWithUpdateSerial returns a manifest with update serial
func (c *Catalog) BOSHManifestWithUpdateSerial() (*manifest.Manifest, error) {
	m, err := manifest.LoadYAML([]byte(bm.BPMReleaseWithUpdateSerial))
	if err != nil {
		return &manifest.Manifest{}, errors.Wrapf(err, manifestFailedMessage)
	}
	return m, nil
}

// BOSHManifestWithUpdateSerialInManifest returns a manifest with update serial in manifest
func (c *Catalog) BOSHManifestWithUpdateSerialInManifest() (*manifest.Manifest, error) {
	m, err := manifest.LoadYAML([]byte(bm.BPMReleaseWithUpdateSerialInManifest))
	if err != nil {
		return &manifest.Manifest{}, errors.Wrapf(err, manifestFailedMessage)
	}
	return m, nil
}

// BOSHManifestWithUpdateSerialAndWithoutPorts returns a manifest with update serial and without ports
func (c *Catalog) BOSHManifestWithUpdateSerialAndWithoutPorts() (*manifest.Manifest, error) {
	m, err := manifest.LoadYAML([]byte(bm.BPMReleaseWithUpdateSerialAndWithoutPorts))
	if err != nil {
		return &manifest.Manifest{}, errors.Wrapf(err, manifestFailedMessage)
	}
	return m, nil
}

// BOSHManifestWithNilConsume returns a manifest with a nil consume for the job
func (c *Catalog) BOSHManifestWithNilConsume() (*manifest.Manifest, error) {
	m, err := manifest.LoadYAML([]byte(bm.WithNilConsume))
	if err != nil {
		return &manifest.Manifest{}, errors.Wrapf(err, manifestFailedMessage)
	}
	return m, nil
}

// NatsPod for use in tests
func (c *Catalog) NatsPod(deployName string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nats",
			Labels: map[string]string{
				bdv1.LabelDeploymentName: deployName,
				"app":                    "nats",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            "nats",
					Image:           "docker.io/bitnami/nats:1.1.0",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Command:         []string{"gnatsd"},
					Args:            []string{"-c", "/opt/bitnami/nats/gnatsd.conf"},
					Ports: []corev1.ContainerPort{
						{
							Name:          "client",
							ContainerPort: 4222,
						},
						{
							Name:          "cluster",
							ContainerPort: 6222,
						},
						{
							Name:          "monitoring",
							ContainerPort: 8222,
						},
					},
					LivenessProbe: &corev1.Probe{
						Handler: corev1.Handler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/",
								Port: intstr.FromString("monitoring"),
							},
						},
						FailureThreshold:    6,
						PeriodSeconds:       10,
						SuccessThreshold:    1,
						TimeoutSeconds:      5,
						InitialDelaySeconds: 30,
					},
					ReadinessProbe: &corev1.Probe{
						Handler: corev1.Handler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/",
								Port: intstr.FromString("monitoring"),
							},
						},
						FailureThreshold:    6,
						PeriodSeconds:       10,
						SuccessThreshold:    1,
						TimeoutSeconds:      5,
						InitialDelaySeconds: 5,
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "config",
							MountPath: "/opt/bitnami/nats/gnatsd.conf",
							SubPath:   "gnatsd.conf",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "config",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "nats",
							},
						},
					},
				},
			},
		},
	}
}

// NatsConfigMap for use in tests
func (c *Catalog) NatsConfigMap(deployName string) corev1.ConfigMap {
	return corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nats",
			Labels: map[string]string{
				bdv1.LabelDeploymentName: deployName,
			},
		},
		Data: map[string]string{
			"gnatsd.conf": `listen: 0.0.0.0:4222
http: 0.0.0.0:8222

# Authorization for client connections
authorization {
  user: nats_client
  password: r9fXAlY3gZ
  timeout:  1
}

# Logging options
debug: false
trace: false
logtime: false

# Pid file
pid_file: "/tmp/gnatsd.pid"

# Some system overides


# Clustering definition
cluster {
  listen: 0.0.0.0:6222

  # Authorization for cluster connections
  authorization {
	user: nats_cluster
	password: hK9awRcEYs
	timeout:  1
  }

  # Routes are actively solicited and connected to from this server.
  # Other servers can connect to us if they supply the correct credentials
  # in their routes definitions from above
  routes = [
	nats://nats_cluster:hK9awRcEYs@nats-headless:6222
  ]
}`,
		},
	}
}

// NatsService for use in tests
func (c *Catalog) NatsService(deployName string) corev1.Service {
	return corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nats-headless",
			Labels: map[string]string{
				bdv1.LabelDeploymentName: deployName,
			},
			Annotations: map[string]string{
				bdv1.AnnotationLinkProviderName: "nats",
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				"app": "nats",
			},
			Ports: []corev1.ServicePort{
				corev1.ServicePort{
					Name:       "client",
					Port:       4222,
					TargetPort: intstr.FromString("client"),
				},
				corev1.ServicePort{
					Name:       "cluster",
					Port:       6222,
					TargetPort: intstr.FromString("cluster"),
				},
			},
		},
	}
}

// NatsSecret for use in tests
func (c *Catalog) NatsSecret(deployName string) corev1.Secret {
	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nats",
			Labels: map[string]string{
				bdv1.LabelDeploymentName: deployName,
			},
			Annotations: map[string]string{
				bdv1.AnnotationLinkProviderName: "nats",
				bdv1.AnnotationLinkProviderType: "nats",
			},
		},
		StringData: map[string]string{
			"user":     "nats_client",
			"password": "r9fXAlY3gZ",
			"port":     "4222",
		},
	}
}
