package boshdns

import (
	"context"
	"fmt"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	bdm "code.cloudfoundry.org/quarks-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/mutate"
)

const (
	appName = "bosh-dns"
)

var (
	boshDNSDockerImage = ""
	clusterDomain      = ""
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

// Alias of domain alias.
type Alias struct {
	Domain  string   `json:"domain"`
	Targets []Target `json:"targets"`
}

// BoshDomainNameService is used to emulate Bosh DNS.
type BoshDomainNameService struct {
	Aliases        []Alias
	LocalDNSIP     string
	InstanceGroups bdm.InstanceGroups
}

// NewBoshDomainNameService create a new DomainNameService to setup BOSH DNS.
func NewBoshDomainNameService(addOn *bdm.AddOn, instanceGroups bdm.InstanceGroups) (*BoshDomainNameService, error) {
	dns := BoshDomainNameService{
		InstanceGroups: instanceGroups,
	}
	for _, job := range addOn.Jobs {
		aliasesProperty := job.Properties.Properties["aliases"]
		if aliasesProperty != nil {
			aliases := make([]Alias, 0)
			if err := mapstructure.Decode(aliasesProperty, &aliases); err != nil {
				return nil, errors.Wrapf(err, "failed to load aliases from manifest")
			}
			dns.Aliases = append(dns.Aliases, aliases...)
		}
	}
	return &dns, nil
}

// DNSSetting see interface.
func (dns *BoshDomainNameService) DNSSetting(namespace string) (corev1.DNSPolicy, *corev1.PodDNSConfig, error) {
	if dns.LocalDNSIP == "" {
		return corev1.DNSNone, nil, errors.New("BoshDomainNameService: DNSSetting called before Apply")
	}
	ndots := "5"
	return corev1.DNSNone, &corev1.PodDNSConfig{
		Nameservers: []string{dns.LocalDNSIP},
		Searches: []string{
			fmt.Sprintf("%s.svc.%s", namespace, clusterDomain),
			fmt.Sprintf("svc.%s", clusterDomain),
			clusterDomain,
		},
		Options: []corev1.PodDNSConfigOption{{Name: "ndots", Value: &ndots}},
	}, nil
}

// Apply DNS k8s resources. This deploys CoreDNS with our DNS records in a config map.
func (dns *BoshDomainNameService) Apply(ctx context.Context, namespace string, c client.Client, setOwner func(object metav1.Object) error) error {
	const volumeName = "bosh-dns-volume"
	const coreConfigFile = "Corefile"

	dnsTCPPort := corev1.ContainerPort{ContainerPort: 8053, Name: "dns-tcp", Protocol: "TCP"}
	dnsUDPPort := corev1.ContainerPort{ContainerPort: 8053, Name: "dns-udp", Protocol: "UDP"}
	metricsPort := corev1.ContainerPort{ContainerPort: 9153, Name: "metrics", Protocol: "TCP"}

	metadata := metav1.ObjectMeta{
		Name:      appName,
		Namespace: namespace,
		Labels:    map[string]string{"app": appName},
	}

	corefile, err := createCorefile(namespace, dns.InstanceGroups, dns.Aliases)
	if err != nil {
		return err
	}

	configMap := corev1.ConfigMap{
		ObjectMeta: metadata,
		Data:       map[string]string{coreConfigFile: corefile},
	}
	service := corev1.Service{
		ObjectMeta: metadata,
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{Name: dnsUDPPort.Name, Port: 53, Protocol: dnsUDPPort.Protocol, TargetPort: intstr.FromString(dnsUDPPort.Name)},
				{Name: dnsTCPPort.Name, Port: 53, Protocol: dnsTCPPort.Protocol, TargetPort: intstr.FromString(dnsTCPPort.Name)},
				{Name: metricsPort.Name, Port: 9153, Protocol: metricsPort.Protocol, TargetPort: intstr.FromString(metricsPort.Name)},
			},
			Selector: map[string]string{"app": appName},
			Type:     "ClusterIP",
		},
	}

	var corefileMode int32 = 0644
	var replicas int32 = 2
	deployment := appsv1.Deployment{
		ObjectMeta: metadata,
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": appName},
			},
			Replicas: &replicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": appName},
				},
				Spec: corev1.PodSpec{
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
										Name: appName,
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
	for _, obj := range []metav1.Object{&configMap, &deployment, &service} {
		if err := setOwner(obj); err != nil {
			return err
		}
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, c, &configMap, configMapMutateFn(&configMap)); err != nil {
		return err
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, c, &deployment, deploymentMapMutateFn(&deployment)); err != nil {
		return err
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, c, &service, mutate.ServiceMutateFn(&service)); err != nil {
		return err
	}

	dns.LocalDNSIP = service.Spec.ClusterIP
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
