package manifest

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"

	v13 "k8s.io/api/apps/v1"

	"k8s.io/apimachinery/pkg/util/intstr"

	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"code.cloudfoundry.org/cf-operator/pkg/kube/util/mutate"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1 "k8s.io/api/core/v1"

	"code.cloudfoundry.org/cf-operator/pkg/kube/util/names"
	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//DomainNameService abstraction
type DomainNameService interface {
	// FindServiceNames determines how a service should be named in accordance with the 'bosh-dns'-addon
	FindServiceNames(instanceGroupName string, deploymentName string) []string

	// HeadlessServiceName constructs the headless service name for the instance group.
	HeadlessServiceName(instanceGroupName string, deploymentName string) string

	// Configure DNS inside pod
	ConfigurePod(podSpec *v1.PodSpec)

	// Reconcile DNS stuff
	Reconcile(ctx context.Context, c client.Client, setOwner func(object v12.Object) error) error
}

// Target of domain alias
type Target struct {
	Query         string `json:"query"`
	InstanceGroup string `json:"instance_group"`
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
	namespace  string
	aliases    []Alias
	localDNSIP string
}

var _ DomainNameService = &boshDomainNameService{}
var simple DomainNameService = &simpleDomainNameService{}

// BoshDNSAddOnName name of bosh dns add on
const BoshDNSAddOnName = "bosh-dns-aliases"

// NewSimpleDomainNameService emulates old behaviour without bosh dns
func NewSimpleDomainNameService() DomainNameService {
	return simple
}

// NewBoshDomainNameService create a new DomainNameService
func NewBoshDomainNameService(namespace string, addOn *AddOn) (DomainNameService, error) {
	dns := boshDomainNameService{namespace: namespace}
	for _, job := range addOn.Jobs {
		aliases := job.Properties.Properties["aliases"]
		if aliases != nil {
			aliasesBytes, err := json.Marshal(aliases)
			if err != nil {
				return nil, errors.Wrapf(err, "Loading aliases from manifest")
			}
			var a = make([]Alias, 0)
			err = json.Unmarshal(aliasesBytes, &a)
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

// ConfigurePod see interface
func (dns *boshDomainNameService) ConfigurePod(podSpec *v1.PodSpec) {
	podSpec.DNSPolicy = v1.DNSNone
	podSpec.DNSConfig = &v1.PodDNSConfig{
		Nameservers: []string{dns.localDNSIP},
		Searches:    []string{fmt.Sprintf("%s.svc.cluster.local", dns.namespace), "service.cf.internal"},
	}
}

// Reconcile see interface
func (dns *boshDomainNameService) Reconcile(ctx context.Context, c client.Client, setOwner func(object v12.Object) error) error {

	rewrites := ""
	for _, alias := range dns.aliases {
		for _, target := range alias.Targets {
			serviceName := serviceName(target.InstanceGroup, dns.namespace, 63)
			rewrites = rewrites + fmt.Sprintf("  rewrite name exact %s %s.%s.svc.cluster.local\n", serviceName, strings.Split(alias.Domain, ".")[0], dns.namespace)
			rewrites = rewrites + fmt.Sprintf("  rewrite name exact %s.%s %s.%s.svc.cluster.local\n", serviceName, dns.namespace, strings.Split(alias.Domain, ".")[0], dns.namespace)
		}
	}
	metadata := v12.ObjectMeta{
		Name:      "bosh-dns",
		Namespace: dns.namespace,
		Labels:    map[string]string{"app": "bosh-dns"},
	}

	configMap := v1.ConfigMap{
		ObjectMeta: metadata,
		Data:       map[string]string{"Corefile": fmt.Sprintf(corefile, rewrites, dns.namespace, dns.rootDNSIP())},
	}
	service := v1.Service{
		ObjectMeta: metadata,
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{Name: "dns", Port: 53, Protocol: "UDP", TargetPort: intstr.FromInt(8053)},
				{Name: "dns-tcp", Port: 53, Protocol: "TCP", TargetPort: intstr.FromInt(8053)},
				{Name: "metrics", Port: 9153, Protocol: "TCP", TargetPort: intstr.FromInt(9153)},
			},
			Selector: map[string]string{"app": "bosh-dns"},
			Type:     "ClusterIP",
		},
	}

	var mode int32 = 420
	deployment := v13.Deployment{
		ObjectMeta: metadata,
		Spec: v13.DeploymentSpec{
			Selector: &v12.LabelSelector{
				MatchLabels: map[string]string{"app": "bosh-dns"},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: v12.ObjectMeta{
					Labels: map[string]string{"app": "bosh-dns"},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "coredns",
							Args:  []string{"-conf", "/etc/coredns/Corefile"},
							Image: "eu.gcr.io/gardener-project/3rd/coredns/coredns:1.4.0",
							Ports: []v1.ContainerPort{
								{ContainerPort: 8053, Name: "dns-udp", Protocol: "UDP"},
								{ContainerPort: 8053, Name: "dns-tcp", Protocol: "TCP"},
								{ContainerPort: 9153, Name: "metrics", Protocol: "TCP"},
							},
							VolumeMounts: []v1.VolumeMount{
								{MountPath: "/etc/coredns", Name: "bosh-dns-volume", ReadOnly: true},
							},
						},
					},
					Volumes: []v1.Volume{
						{
							Name: "bosh-dns-volume",
							VolumeSource: v1.VolumeSource{
								ConfigMap: &v1.ConfigMapVolumeSource{
									DefaultMode: &mode,
									LocalObjectReference: v1.LocalObjectReference{
										Name: "bosh-dns",
									},
									Items: []v1.KeyToPath{
										{Key: "Corefile", Path: "Corefile"},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	for _, obj := range []v12.Object{&configMap, &deployment, &service} {
		err := setOwner(obj)
		if err != nil {
			return err
		}
	}

	_, err := controllerutil.CreateOrUpdate(ctx, c, &configMap, configMapMutateFn(&configMap))
	if err != nil {
		return err
	}
	_, err = controllerutil.CreateOrUpdate(ctx, c, &deployment, deploymentMapMutateFn(&deployment))
	if err != nil {
		return err
	}
	_, err = controllerutil.CreateOrUpdate(ctx, c, &service, mutate.ServiceMutateFn(&service))
	if err != nil {
		return err
	}

	dns.localDNSIP = service.Spec.ClusterIP
	return err
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
		return "1.1.1.1"
	}
	return string(found[1])

}

type simpleDomainNameService struct {
}

// FindServiceNames see interface
func (dns *simpleDomainNameService) FindServiceNames(instanceGroupName string, deploymentName string) []string {
	return []string{serviceName(instanceGroupName, deploymentName, 63)}
}

// HeadlessServiceName see interface
func (dns *simpleDomainNameService) HeadlessServiceName(instanceGroupName string, deploymentName string) string {
	return serviceName(instanceGroupName, deploymentName, 63)
}

// ConfigurePod see interface
func (dns *simpleDomainNameService) ConfigurePod(podSpec *v1.PodSpec) {
}

// Reconcile see interface
func (dns *simpleDomainNameService) Reconcile(ctx context.Context, c client.Client, setOwner func(object v12.Object) error) error {
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

func configMapMutateFn(configMap *v1.ConfigMap) controllerutil.MutateFn {
	updated := configMap.DeepCopy()
	return func() error {
		configMap.Labels = updated.Labels
		configMap.Annotations = updated.Annotations
		configMap.Data = updated.Data
		return nil
	}
}

func deploymentMapMutateFn(deployment *v13.Deployment) controllerutil.MutateFn {
	updated := deployment.DeepCopy()
	return func() error {
		deployment.Labels = updated.Labels
		deployment.Annotations = updated.Annotations
		deployment.Spec = updated.Spec
		return nil
	}
}

const corefile = `
.:8053 {
  health
  %s
  rewrite name substring service.cf.internal %s.svc.cluster.local
  forward . %s
}
`
