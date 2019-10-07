package manifest

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"

	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"

	appsv1 "k8s.io/api/apps/v1"

	"k8s.io/apimachinery/pkg/util/intstr"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"code.cloudfoundry.org/cf-operator/pkg/kube/util/mutate"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	corev1 "k8s.io/api/core/v1"

	"code.cloudfoundry.org/cf-operator/pkg/kube/util/names"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//DomainNameService abstraction
type DomainNameService interface {
	// FindServiceNames determines how a service should be named in accordance with the 'bosh-dns'-addon
	FindServiceNames(instanceGroupName string, deploymentName string) []string

	// HeadlessServiceName constructs the headless service name for the instance group.
	HeadlessServiceName(instanceGroupName string, deploymentName string) string

	// DNSSetting get the DNS settings for POD
	DNSSetting() (corev1.DNSPolicy, *corev1.PodDNSConfig)

	// Reconcile DNS stuff
	Reconcile(ctx context.Context, namespace string, manifestName string, c client.Client, setOwner func(object metav1.Object) error) error
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
	aliases    []Alias
	localDNSIP string
}

var _ DomainNameService = &boshDomainNameService{}

// BoshDNSAddOnName name of bosh dns add on
const BoshDNSAddOnName = "bosh-dns-aliases"

// NewBoshDomainNameService create a new DomainNameService
func NewBoshDomainNameService(addOn *AddOn) (DomainNameService, error) {
	dns := boshDomainNameService{}
	for _, job := range addOn.Jobs {
		aliases := job.Properties.Properties["aliases"]
		if aliases != nil {
			var a = make([]Alias, 0)
			err := mapstructure.Decode(aliases, &a)
			if err != nil {
				return nil, errors.Wrapf(err, "Loading aliases from manifest")
			}
			dns.aliases = append(dns.aliases, a...)
		}
	}
	return &dns, nil
}

// FindServiceNames see interface
func (dns *boshDomainNameService) FindServiceNames(instanceGroupName string, deploymentName string) []string {
	result := make([]string, 0)
	for _, alias := range dns.aliases {
		for _, target := range alias.Targets {
			if target.InstanceGroup == instanceGroupName {
				result = append(result, strings.Split(alias.Domain, ".")[0])
			}
		}
	}
	if len(result) == 0 {
		result = append(result, serviceName(instanceGroupName, deploymentName, 63))
	}
	return result
}

// HeadlessServiceName see interface
func (dns *boshDomainNameService) HeadlessServiceName(instanceGroupName string, deploymentName string) string {
	serviceNames := dns.FindServiceNames(instanceGroupName, deploymentName)
	if len(serviceNames) == 0 {
		return serviceName(instanceGroupName, deploymentName, 63)
	}
	return serviceNames[0]
}

// DNSSetting see interface
func (dns *boshDomainNameService) DNSSetting() (corev1.DNSPolicy, *corev1.PodDNSConfig) {
	if dns.localDNSIP == "" {
		panic("BoshDomainNameService: DNSSetting called before Reconcile")
	}
	ndots := "5"
	return corev1.DNSNone, &corev1.PodDNSConfig{
		Nameservers: []string{dns.localDNSIP},
		Searches:    []string{"svc.cluster.local", "cluster.local", "service.cf.internal"},
		Options:     []corev1.PodDNSConfigOption{{Name: "ndots", Value: &ndots}},
	}
}

// Reconcile see interface
func (dns *boshDomainNameService) Reconcile(ctx context.Context, namespace string, manifestName string, c client.Client, setOwner func(object metav1.Object) error) error {
	const appName = "bosh-dns"
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

	configMap := corev1.ConfigMap{
		ObjectMeta: metadata,
		Data:       map[string]string{coreConfigFile: createCorefile(dns, namespace, manifestName)},
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
							Image: "coredns/coredns:1.6.3",
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

	dns.localDNSIP = service.Spec.ClusterIP
	return nil
}

var re = regexp.MustCompile(`nameserver\s+([\d\.]*)`)

func (dns *boshDomainNameService) rootDNSIP() string {
	content, err := ioutil.ReadFile("/etc/resolv.conf")
	if err != nil {
		content = []byte{}
	}
	return GetNameserverFromResolveConfig(content)
}

// GetNameserverFromResolveConfig read nameserver from resolve.conf
func GetNameserverFromResolveConfig(content []byte) string {
	found := re.FindSubmatch(content)
	if len(found) == 0 {
		ctxlog.Errorf(context.Background(), "No nameserver found in resolve.conf using 1.1.1.1")
		return "1.1.1.1"
	}
	return string(found[1])

}

type simpleDomainNameService struct {
}

var _ DomainNameService = &simpleDomainNameService{}

// NewSimpleDomainNameService emulates old behaviour without bosh dns
func NewSimpleDomainNameService() DomainNameService {
	return &simpleDomainNameService{}
}

// FindServiceNames see interface
func (dns *simpleDomainNameService) FindServiceNames(instanceGroupName string, deploymentName string) []string {
	return []string{serviceName(instanceGroupName, deploymentName, 63)}
}

// HeadlessServiceName see interface
func (dns *simpleDomainNameService) HeadlessServiceName(instanceGroupName string, deploymentName string) string {
	return serviceName(instanceGroupName, deploymentName, 63)
}

// DNSSetting see interface
func (dns *simpleDomainNameService) DNSSetting() (corev1.DNSPolicy, *corev1.PodDNSConfig) {
	return corev1.DNSClusterFirst, nil
}

// Reconcile see interface
func (dns *simpleDomainNameService) Reconcile(ctx context.Context, namespace string, manifestName string, c client.Client, setOwner func(object metav1.Object) error) error {
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

func createCorefile(dns *boshDomainNameService, namespace string, manifestName string) string {
	rewrites := ""

	// Support legacy cf-operator naming convention: <deployment name>-<ig name>
	for _, alias := range dns.aliases {
		newDomainName := fmt.Sprintf(`%s.%s.svc.cluster.local`, strings.Split(alias.Domain, ".")[0], namespace)
		for _, target := range alias.Targets {
			serviceName := serviceName(target.InstanceGroup, manifestName, 63)
			// For backwards compatibility, e.g. 'scf-nats'
			rewrites = rewrites + fmt.Sprintf(`    rewrite name exact %s %s`,
				regexp.QuoteMeta(serviceName), newDomainName) + "\n"
			rewrites = rewrites + fmt.Sprintf(`    rewrite name exact %s.%s.svc.cluster.local %s`,
				regexp.QuoteMeta(serviceName), regexp.QuoteMeta(namespace), newDomainName) + "\n"
		}
	}
	// Kubernetes services should stay resolvable
	rewrites = rewrites + fmt.Sprintf(`    rewrite name regex ^([^.]+)$    {1}.%s.svc.cluster.local.`, namespace) + "\n"

	return fmt.Sprintf(`
.:8053 {
    health
%s
    rewrite name substring service.cf.internal. %s.svc.cluster.local.
    forward . %s
}
	`, rewrites, namespace, dns.rootDNSIP())
}
