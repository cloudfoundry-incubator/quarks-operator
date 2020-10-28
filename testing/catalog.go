// Package testing contains methods to create test data. It's a seaparate
// package to avoid import cycles. Helper functions can be found in the package
// `testhelper`.
package testing

import (
	"encoding/json"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/afero"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"code.cloudfoundry.org/quarks-operator/pkg/bosh/bpmconverter"
	"code.cloudfoundry.org/quarks-operator/pkg/bosh/manifest"
	bdv1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/controllers/waitservice"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/names"
	bm "code.cloudfoundry.org/quarks-operator/testing/boshmanifest"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/credsgen"
	sharednames "code.cloudfoundry.org/quarks-utils/pkg/names"
	"code.cloudfoundry.org/quarks-utils/pkg/pointers"
	"code.cloudfoundry.org/quarks-utils/pkg/versionedsecretstore"
)

const (
	manifestFailedMessage = "Loading bosh manifest spec failed."
	logsDir               = "LOGS_DIR"
	logsDirPath           = "/var/vcap/sys/log"
)

// Catalog provides several instances for tests
type Catalog struct{}

// DefaultConfig for tests
func (c *Catalog) DefaultConfig() *config.Config {
	return &config.Config{
		CtxTimeOut:        10 * time.Second,
		OperatorNamespace: "default",
		MonitoredID:       "staging",
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

// BOSHManifestFromKubeCF641 for "real-life" tests
func (c *Catalog) BOSHManifestFromKubeCF641() (*manifest.Manifest, error) {
	m, err := manifest.LoadYAML([]byte(bm.FromKubeCF641))
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

// BPMReleaseWithTolerations returns a manifest with tolerations
func (c *Catalog) BPMReleaseWithTolerations() (*manifest.Manifest, error) {
	m, err := manifest.LoadYAML([]byte(bm.BPMReleaseWithTolerations))
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

// BOSHManifestWithActivePassiveProbes returns a manifest with an active/passive probe
func (c *Catalog) BOSHManifestWithActivePassiveProbes() (*manifest.Manifest, error) {
	m, err := manifest.LoadYAML([]byte(bm.WithActivePassiveProbes))
	if err != nil {
		return &manifest.Manifest{}, errors.Wrapf(err, manifestFailedMessage)
	}
	return m, nil
}

// BOSHManifestSecret returns a secret containing the BOSH manifest
func (c *Catalog) BOSHManifestSecret(name string, text string) corev1.Secret {
	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		StringData: map[string]string{
			"manifest": text,
		},
	}
}

// BOSHManifestConfigMap creates a config map containing the BOSH manifest
func (c *Catalog) BOSHManifestConfigMap(name string, text string) corev1.ConfigMap {
	return corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Data: map[string]string{
			"manifest": text,
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

// VersionedSecret for tests
func (c *Catalog) VersionedSecret(name string) corev1.Secret {
	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				versionedsecretstore.LabelSecretKind: versionedsecretstore.VersionSecretKind,
			},
		},
		StringData: map[string]string{
			name: "default-value",
		},
	}
}

// QuarksLinkSecret returns a link secret, as generated for consumption by an external (non BOSH) consumer
func (c *Catalog) QuarksLinkSecret(deploymentName, linkType, linkName string, value map[string][]byte) corev1.Secret {
	name := names.QuarksLinkSecretName(linkType, linkName)
	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Annotations: map[string]string{},
			Labels: map[string]string{
				bdv1.LabelDeploymentName:  deploymentName,
				bdv1.LabelEntanglementKey: name,
			},
		},
		Data: value,
	}
}

// DefaultQuarksLinkSecret has default values from the nats release
func (c *Catalog) DefaultQuarksLinkSecret(deploymentName, linkType string) corev1.Secret {
	return c.QuarksLinkSecret(
		deploymentName,
		linkType,
		"nats",
		map[string][]byte{
			"nats.password": []byte("custom_password"),
			"nats.port":     []byte("4222"),
			"nats.user":     []byte("admin"),
		},
	)
}

// CustomOpsConfigMap is an operations file with a custom structural change
func (c *Catalog) CustomOpsConfigMap(name string, change string) corev1.ConfigMap {
	return corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Data: map[string]string{
			"ops": change,
		},
	}
}

// UserExplicitPassword is a secret representing an explicit var, used as a password
func (c *Catalog) UserExplicitPassword(name, password string) corev1.Secret {
	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		StringData: map[string]string{
			"password": password,
		},
	}
}

// CustomOpsSecret is an operations file with a custom structural change
func (c *Catalog) CustomOpsSecret(name string, change string) corev1.Secret {
	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		StringData: map[string]string{
			"ops": change,
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

// DefaultPod defines a pod with a simple web server useful for testing
func (c *Catalog) DefaultPod(name string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
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
			"quarks.cloudfoundry.org/consumes":   `[{"name":"nats","type":"nats"}]`,
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

// PodWithTailLogsContainer will generate a pod with two containers
// One is the parent container that will execute a cmd, preferrable something
// that writes into files under /var/vcap/sys/log
// The side-car container, will be tailing the logs of specific files under
// /var/vcap/sys/log, by running the quarks-operator util tail-logs subcmommand
func (c *Catalog) PodWithTailLogsContainer(podName string, parentPodCmd string, parentCName string, sidecardCName string, dockerImg string) corev1.Pod {
	rootUserID := int64(0)

	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyOnFailure,
			Containers: []corev1.Container{
				{
					Name:  parentCName,
					Image: dockerImg,
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      bpmconverter.VolumeSysDirName,
							MountPath: bpmconverter.VolumeSysDirMountPath,
						},
					},
					Command: []string{
						"/bin/sh",
					},
					Args: []string{
						"-xc",
						parentPodCmd,
					},
				},
				{
					Name:  sidecardCName,
					Image: dockerImg,
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      bpmconverter.VolumeSysDirName,
							MountPath: bpmconverter.VolumeSysDirMountPath,
						},
					},
					Args: []string{
						"util",
						"tail-logs",
					},
					Env: []corev1.EnvVar{
						{
							Name:  logsDir,
							Value: logsDirPath,
						},
					},
					SecurityContext: &corev1.SecurityContext{
						RunAsUser: &rootUserID,
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name:         bpmconverter.VolumeSysDirName,
					VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
				},
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
				sharednames.GroupName + "/instance-group-name": ig,
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

// WaitingPod is a pod which has the wait-for annotation for use in tests
func (c *Catalog) WaitingPod(name string, serviceList ...string) corev1.Pod {
	services, _ := json.Marshal(serviceList)
	return c.AnnotatedPod(name, map[string]string{
		waitservice.WaitKey: string(services),
	})
}

// EchoContainer returns a container which just "echo" for use in tests
func (c *Catalog) EchoContainer(name string) corev1.Container {
	return corev1.Container{
		Name:    name,
		Image:   "busybox",
		Command: []string{"echo"},
	}
}

// DummyService for use in tests
func (c *Catalog) DummyService(serviceName string) corev1.Service {
	return corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceName,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
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
