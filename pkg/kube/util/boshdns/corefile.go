package boshdns

import (
	"fmt"
	"regexp"
	"strings"
	"text/template"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"

	bdm "code.cloudfoundry.org/quarks-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/names"
)

// Corefile contains config for coredns
type Corefile struct {
	Aliases  []Alias   `json:"aliases"`
	Handlers []Handler `json:"handlers"`
}

// Alias of domain alias.
type Alias struct {
	Domain  string   `json:"domain"`
	Targets []Target `json:"targets"`
}

// Handler redirects DNS queries for a zone to a forward server
// https://coredns.io/plugins/forward/
type Handler struct {
	Domain string        `json:"domain"`
	Source HandlerSource `json:"source"`
}

// Zone returns the domain with the trailing dot removed
func (h Handler) Zone() string {
	return strings.TrimRight(h.Domain, ".")
}

// HandlerSource points to the forward DNS server
type HandlerSource struct {
	Recursors []string `json:"recursors"`
	Type      string   `json:"type"`
}

// Protocol returns the coredns server protocol.
// https://coredns.io/manual/toc/#specifying-a-protocol
func (h HandlerSource) Protocol() string {
	switch h.Type {
	case "tls":
		return "tls://"
	case "https", "http":
		return "https://"
	case "grpc":
		return "grpc://"
	default:
		return "dns://"
	}
}

// Add an entry (alias or handler) to the corefile
func (c *Corefile) Add(props map[string]interface{}) error {
	tmp := &Corefile{}
	if err := mapstructure.Decode(props, tmp); err != nil {
		return errors.Wrapf(err, "failed to load dns addon config")
	}
	c.Aliases = append(c.Aliases, tmp.Aliases...)
	c.Handlers = append(c.Handlers, tmp.Handlers...)

	return nil
}

// Create the coredns corefile
func (c *Corefile) Create(namespace string, instanceGroups bdm.InstanceGroups) (string, error) {
	rewrites := make([]string, 0)
	for _, alias := range c.Aliases {
		for _, target := range alias.Targets {
			// Implement BOSH DNS placeholder alias: https://bosh.io/docs/dns/#placeholder-alias.
			instanceGroup, found := instanceGroups.InstanceGroupByName(target.InstanceGroup)
			if !found {
				// Even if the instance group doesn't exist, the user may want to setup aliases to other kube service names
				rewrites = gatherSimpleRewrites(rewrites,
					target,
					namespace,
					alias,
				)
			} else {
				rewrites = gatherAllRewrites(rewrites,
					*instanceGroup,
					target,
					namespace,
					alias)
			}
		}
	}

	tmpl := template.Must(template.New("Corefile").Parse(corefileTemplate))
	var config strings.Builder
	data := struct {
		Rewrites []string
		Handlers []Handler
	}{rewrites, c.Handlers}
	if err := tmpl.Execute(&config, data); err != nil {
		return "", errors.Wrapf(err, "failed to generate Corefile")
	}

	return config.String(), nil
}

func gatherSimpleRewrites(rewrites []string,
	target Target,
	namespace string,
	alias Alias) []string {

	// We can't do simple rewrites for indexes
	if target.Query == "_" {
		return rewrites
	}

	from := alias.Domain
	to := fmt.Sprintf("%s.%s.svc.%s", target.InstanceGroup, namespace, clusterDomain)
	rewrites = append(rewrites, newTemplate(from, to, target.Query))

	return rewrites
}

func gatherAllRewrites(rewrites []string,
	instanceGroup bdm.InstanceGroup,
	target Target,
	namespace string,
	alias Alias) []string {
	if target.Query == "_" {
		if len(instanceGroup.AZs) > 0 {
			for azIndex := range instanceGroup.AZs {
				rewrites = gatherRewritesForInstances(rewrites,
					instanceGroup,
					target,
					namespace,
					azIndex,
					alias)
			}
		} else {
			rewrites = gatherRewritesForInstances(rewrites,
				instanceGroup,
				target,
				namespace,
				-1,
				alias)
		}
	} else {
		from := alias.Domain
		to := fmt.Sprintf("%s.%s.svc.%s",
			names.ServiceName(target.InstanceGroup),
			namespace,
			clusterDomain)
		rewrites = append(rewrites, newTemplate(from, to, target.Query))
	}

	return rewrites
}

func gatherRewritesForInstances(rewrites []string,
	instanceGroup bdm.InstanceGroup,
	target Target,
	namespace string,
	azIndex int,
	alias Alias) []string {
	id := ""
	for i := 0; i < instanceGroup.Instances; i++ {
		if azIndex > -1 {
			id = fmt.Sprintf("%s-z%d-%d", target.InstanceGroup, azIndex, i)
		} else {
			id = fmt.Sprintf("%s-%d", target.InstanceGroup, i)
		}
		from := strings.Replace(alias.Domain, "_", id, 1)
		serviceName := instanceGroup.IndexedServiceName(i, azIndex)
		to := fmt.Sprintf("%s.%s.svc.%s", serviceName, namespace, clusterDomain)
		rewrites = append(rewrites, newTemplate(from, to, target.Query))
	}

	return rewrites
}

// The Corefile values other than the rewrites were based on the default cluster CoreDNS Corefile.
const corefileTemplate = `
{{- range $h := .Handlers }}
{{ .Zone }}:8053 {
	forward . {{ range .Source.Recursors }}{{ $h.Source.Protocol }}{{ . }} {{ end }}
}
{{- end }}
.:8053 {
	errors
	health
	{{- range $rewrite := .Rewrites }}
	{{ $rewrite }}
	{{- end }}
	forward . /etc/resolv.conf
	cache 30
	loop
	reload
	loadbalance
}`

func newTemplate(from, to, queryType string) string {
	return fmt.Sprintf(cnameTemplate, regexp.QuoteMeta(from), "", to, from)
}

const cnameTemplate = `
	template IN A %[4]s {
		match ^%[2]s%[1]s\.$
		answer "{{ .Name }} 60 IN CNAME %[3]s"
		upstream
	}
	template IN AAAA %[4]s {
		match ^%[2]s%[1]s\.$
		answer "{{ .Name }} 60 IN CNAME %[3]s"
		upstream
	}
	template IN CNAME %[4]s {
		match ^%[2]s%[1]s\.$
		answer "{{ .Name }} 60 IN CNAME %[3]s"
		upstream
	}`
