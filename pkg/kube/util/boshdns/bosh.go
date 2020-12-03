package boshdns

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	bdm "code.cloudfoundry.org/quarks-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/apis"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/mutate"
)

const (
	// AppName is the name os the DNS deployed by quarks.
	AppName        = "coredns-quarks"
	coreConfigFile = "Corefile"
	// CorednsServiceAccountLabel is the label of coredns service account on ns.
	CorednsServiceAccountLabel = "quarks.cloudfoundry.org/coredns-quarks-service-account"
)

var (
	annotationRestartOnUpdate = fmt.Sprintf("%s/restart-on-update", apis.GroupName)
	boshDNSDockerImage        = ""
	clusterDomain             = ""
	dnsTCPPort                = corev1.ContainerPort{ContainerPort: 8053, Name: "dns-tcp", Protocol: "TCP"}
	dnsUDPPort                = corev1.ContainerPort{ContainerPort: 8053, Name: "dns-udp", Protocol: "UDP"}
	metricsPort               = corev1.ContainerPort{ContainerPort: 9153, Name: "metrics", Protocol: "TCP"}
)

// SetBoshDNSDockerImage initializes the package scoped boshDNSDockerImage variable.
func SetBoshDNSDockerImage(image string) {
	boshDNSDockerImage = image
}

// SetClusterDomain initializes the package scoped clusterDomain variable.
func SetClusterDomain(domain string) {
	clusterDomain = domain
}

// GetClusterDomain returns the package scoped clusterDomain variable.
func GetClusterDomain() string {
	return clusterDomain
}

// Target of domain alias.
type Target struct {
	Query         string `json:"query"`
	InstanceGroup string `json:"instance_group" mapstructure:"instance_group"`
	Deployment    string `json:"deployment"`
	Network       string `json:"network"`
	Domain        string `json:"domain"`
}

// BoshDomainNameService is used to emulate Bosh DNS.
type BoshDomainNameService struct {
	Corefile       *Corefile
	LocalDNSIP     string
	InstanceGroups bdm.InstanceGroups
}

// NewBoshDomainNameService create a new DomainNameService to setup BOSH DNS.
func NewBoshDomainNameService(instanceGroups bdm.InstanceGroups) *BoshDomainNameService {
	return &BoshDomainNameService{
		Corefile:       &Corefile{},
		InstanceGroups: instanceGroups,
	}
}

// Add create a new DomainNameService to setup BOSH DNS.
func (dns *BoshDomainNameService) Add(addOn *bdm.AddOn) error {
	for _, job := range addOn.Jobs {
		if err := dns.Corefile.Add(job.Properties.Properties); err != nil {
			return err
		}
	}
	return nil
}

// CorefileConfigMap is a ConfigMap that contains the corefile
func (dns *BoshDomainNameService) CorefileConfigMap(namespace string) (corev1.ConfigMap, error) {
	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      AppName,
			Namespace: namespace,
			Labels:    map[string]string{"app": AppName},
		},
	}

	corefile, err := dns.Corefile.Create(namespace, dns.InstanceGroups)
	if err != nil {
		return cm, err
	}
	cm.Data = map[string]string{coreConfigFile: corefile}

	return cm, nil
}

// Deployment returns the k8s Deployment for coredns
func (dns *BoshDomainNameService) Deployment(namespace string, corednsServiceAccountName string) appsv1.Deployment {
	var corefileMode int32 = 0644
	var replicas int32 = 2
	const volumeName = "bosh-dns-volume"
	return appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      AppName,
			Namespace: namespace,
			Labels:    map[string]string{"app": AppName},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": AppName},
			},
			Replicas: &replicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      map[string]string{"app": AppName},
					Annotations: map[string]string{annotationRestartOnUpdate: "true"},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: corednsServiceAccountName,
					Containers: []corev1.Container{
						{
							Name:  "coredns",
							Args:  []string{"-conf", "/etc/coredns/Corefile"},
							Image: boshDNSDockerImage,
							Ports: []corev1.ContainerPort{dnsUDPPort, dnsTCPPort, metricsPort},
							VolumeMounts: []corev1.VolumeMount{
								{MountPath: "/etc/coredns", Name: volumeName, ReadOnly: true},
							},
							ReadinessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/health",
										Port:   intstr.FromInt(8080),
										Scheme: "HTTP",
									},
								},
								FailureThreshold: 3,
								PeriodSeconds:    10,
								SuccessThreshold: 1,
								TimeoutSeconds:   1,
							},
							LivenessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/health",
										Port:   intstr.FromInt(8080),
										Scheme: "HTTP",
									},
								},
								FailureThreshold:    5,
								PeriodSeconds:       10,
								SuccessThreshold:    1,
								TimeoutSeconds:      5,
								InitialDelaySeconds: 60,
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: volumeName,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									DefaultMode: &corefileMode,
									LocalObjectReference: corev1.LocalObjectReference{
										Name: AppName,
									},
									Items: []corev1.KeyToPath{
										{Key: coreConfigFile, Path: coreConfigFile},
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

// Service returns the k8s service to access coredns
func (dns *BoshDomainNameService) Service(namespace string) corev1.Service {
	return corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      AppName,
			Namespace: namespace,
			Labels:    map[string]string{"app": AppName},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{Name: dnsUDPPort.Name, Port: 53, Protocol: dnsUDPPort.Protocol, TargetPort: intstr.FromString(dnsUDPPort.Name)},
				{Name: dnsTCPPort.Name, Port: 53, Protocol: dnsTCPPort.Protocol, TargetPort: intstr.FromString(dnsTCPPort.Name)},
				{Name: metricsPort.Name, Port: 9153, Protocol: metricsPort.Protocol, TargetPort: intstr.FromString(metricsPort.Name)},
			},
			Selector: map[string]string{"app": AppName},
			Type:     "ClusterIP",
		},
	}
}

// Apply DNS k8s resources. This deploys CoreDNS with our DNS records in a config map.
func (dns *BoshDomainNameService) Apply(ctx context.Context, namespace string, c client.Client, setOwner func(object metav1.Object) error) error {
	configMap, err := dns.CorefileConfigMap(namespace)
	if err != nil {
		return err
	}

	var ns corev1.Namespace
	err = c.Get(ctx, client.ObjectKey{Name: namespace}, &ns)
	if err != nil {
		return errors.Wrapf(err, "could not get ns '%s'", namespace)
	}

	corednsServiceAccountName, ok := ns.Labels[CorednsServiceAccountLabel]
	if !ok {
		return errors.Wrapf(err, "could not get coredns service account name from ns '%s'", namespace)
	}

	deployment := dns.Deployment(namespace, corednsServiceAccountName)
	service := dns.Service(namespace)

	for _, obj := range []metav1.Object{&configMap, &deployment, &service} {
		if err := setOwner(obj); err != nil {
			return err
		}
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, c, &configMap, configMapMutateFn(&configMap)); err != nil {
		return err
	}
	if _, err = controllerutil.CreateOrUpdate(ctx, c, &deployment, deploymentMapMutateFn(&deployment)); err != nil {
		return err
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, c, &service, mutate.ServiceMutateFn(&service)); err != nil {
		return err
	}

	return nil
}

func configMapMutateFn(configMap *corev1.ConfigMap) controllerutil.MutateFn {
	updated := configMap.DeepCopy()
	return func() error {
		configMap.Labels = updated.Labels
		configMap.Annotations = updated.Annotations
		configMap.Data = updated.Data
		return nil
	}
}

func deploymentMapMutateFn(deployment *appsv1.Deployment) controllerutil.MutateFn {
	updated := deployment.DeepCopy()
	return func() error {
		deployment.Labels = updated.Labels
		deployment.Annotations = updated.Annotations
		deployment.Spec = updated.Spec
		return nil
	}
}
