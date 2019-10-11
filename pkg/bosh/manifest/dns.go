package manifest

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"code.cloudfoundry.org/cf-operator/pkg/kube/util/mutate"
	"code.cloudfoundry.org/quarks-utils/pkg/names"
)

//DomainNameService abstraction
type DomainNameService interface {
	// HeadlessServiceName constructs the headless service name for the instance group.
	HeadlessServiceName(instanceGroupName string) string

	// DNSSetting get the DNS settings for POD
	DNSSetting(namespace string) (corev1.DNSPolicy, *corev1.PodDNSConfig, error)

	// Reconcile DNS stuff
	Reconcile(ctx context.Context, namespace string, c client.Client, setOwner func(object metav1.Object) error) error
}

// Target of domain alias
type Target struct {
	Query         string `json:"query"`
	InstanceGroup string `json:"instance_group" mapstructure:"instance_group"`
	Deployment    string `json:"deployment"`
	Network       string `json:"network"`
	Domain        string `json:"domain"`
}

// Alias of domain alias
type Alias struct {
	Domain  string   `json:"domain"`
	Targets []Target `json:"targets"`
}

// boshDomainNameService is used to emulate Bosh DNS
type boshDomainNameService struct {
	Aliases      []Alias
	LocalDNSIP   string
	ManifestName string
}

// BoshDNSAddOnName name of bosh dns add on
const BoshDNSAddOnName = "bosh-dns-aliases"

// NewBoshDomainNameService create a new DomainNameService
func NewBoshDomainNameService(addOn *AddOn, manifestName string) (DomainNameService, error) {
	dns := boshDomainNameService{ManifestName: manifestName}
	for _, job := range addOn.Jobs {
		aliases := job.Properties.Properties["aliases"]
		if aliases != nil {
			var a = make([]Alias, 0)
			err := mapstructure.Decode(aliases, &a)
			if err != nil {
				return nil, errors.Wrapf(err, "Loading aliases from manifest")
			}
			dns.Aliases = append(dns.Aliases, a...)
		}
	}
	return &dns, nil
}

// HeadlessServiceName see interface
func (dns *boshDomainNameService) HeadlessServiceName(instanceGroupName string) string {
	return serviceName(instanceGroupName, dns.ManifestName, 63)
}

// DNSSetting see interface
func (dns *boshDomainNameService) DNSSetting(namespace string) (corev1.DNSPolicy, *corev1.PodDNSConfig, error) {
	if dns.LocalDNSIP == "" {
		return corev1.DNSNone, nil, errors.New("BoshDomainNameService: DNSSetting called before Reconcile")
	}
	ndots := "5"
	return corev1.DNSNone, &corev1.PodDNSConfig{
		Nameservers: []string{dns.LocalDNSIP},
		Searches: []string{
			fmt.Sprintf("%s.svc.cluster.local", namespace),
			"svc.cluster.local",
			"cluster.local",
			"service.cf.internal",
		},
		Options: []corev1.PodDNSConfigOption{{Name: "ndots", Value: &ndots}},
	}, nil
}

var boshDNSDockerImage = ""

// SetupBoshDNSDockerImage initializes the package scoped variable
func SetupBoshDNSDockerImage(image string) {
	boshDNSDockerImage = image
}

// Reconcile see interface
func (dns *boshDomainNameService) Reconcile(ctx context.Context, namespace string, c client.Client, setOwner func(object metav1.Object) error) error {
	const volumeName = "bosh-dns-volume"
	const coreConfigFile = "Corefile"

	appName := fmt.Sprintf("%s-bosh-dns", dns.ManifestName)

	dnsTCPPort := corev1.ContainerPort{ContainerPort: 8053, Name: "dns-tcp", Protocol: "TCP"}
	dnsUDPPort := corev1.ContainerPort{ContainerPort: 8053, Name: "dns-udp", Protocol: "UDP"}
	metricsPort := corev1.ContainerPort{ContainerPort: 9153, Name: "metrics", Protocol: "TCP"}

	metadata := metav1.ObjectMeta{
		Name:      appName,
		Namespace: namespace,
		Labels:    map[string]string{"app": appName},
	}

	configMap := corev1.ConfigMap{
		ObjectMeta: metadata,
		Data:       map[string]string{coreConfigFile: createCorefile(dns, namespace)},
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

	var mode int32 = 420
	deployment := appsv1.Deployment{
		ObjectMeta: metadata,
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": appName},
			},
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
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: volumeName,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									DefaultMode: &mode,
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

type simpleDomainNameService struct {
	ManifestName string
}

// NewSimpleDomainNameService emulates old behaviour without bosh dns
func NewSimpleDomainNameService(manifestName string) DomainNameService {
	return &simpleDomainNameService{ManifestName: manifestName}
}

// HeadlessServiceName see interface
func (dns *simpleDomainNameService) HeadlessServiceName(instanceGroupName string) string {
	return serviceName(instanceGroupName, dns.ManifestName, 63)
}

// DNSSetting see interface
func (dns *simpleDomainNameService) DNSSetting(_ string) (corev1.DNSPolicy, *corev1.PodDNSConfig, error) {
	return corev1.DNSClusterFirst, nil, nil
}

// Reconcile see interface
func (dns *simpleDomainNameService) Reconcile(ctx context.Context, namespace string, c client.Client, setOwner func(object metav1.Object) error) error {
	return nil
}

func serviceName(instanceGroupName string, deploymentName string, maxLength int) string {
	serviceName := fmt.Sprintf("%s-%s", deploymentName, names.Sanitize(instanceGroupName))
	if len(serviceName) > maxLength {
		sumHex := md5.Sum([]byte(serviceName))
		sum := hex.EncodeToString(sumHex[:])
		serviceName = fmt.Sprintf("%s-%s", serviceName[:maxLength-len(sum)-1], sum)
	}
	return serviceName
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

func createCorefile(dns *boshDomainNameService, namespace string) string {
	var config strings.Builder

	writeln := func(indent int, line string) {
		fmt.Fprintf(&config, "%s%s\n", strings.Repeat(" ", indent), line)
	}

	indent := 0
	writeln(indent, ".:8053 {")

	indent = 4
	writeln(indent, "errors")
	writeln(indent, "health")

	for _, alias := range dns.Aliases {
		for _, target := range alias.Targets {
			from := alias.Domain
			if target.Query == "_" {
				from = strings.Replace(from, "_", target.InstanceGroup, 1)
			}
			to := fmt.Sprintf("%s.%s.svc.cluster.local", dns.HeadlessServiceName(target.InstanceGroup), namespace)
			writeln(indent, fmt.Sprintf("rewrite name exact %s %s", from, to))
		}
	}

	writeln(indent, "forward . /etc/resolv.conf")
	writeln(indent, "cache 30")
	writeln(indent, "loop")
	writeln(indent, "reload")
	writeln(indent, "loadbalance")

	indent = 0
	writeln(indent, "}")

	return config.String()
}
