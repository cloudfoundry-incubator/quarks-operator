package boshdns

import (
	"fmt"
	"regexp"
	"strings"
	"text/template"

	"github.com/pkg/errors"

	bdm "code.cloudfoundry.org/quarks-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/names"
)

func createCorefile(namespace string, instanceGroups bdm.InstanceGroups, aliases []Alias) (string, error) {
	rewrites := make([]string, 0)
	for _, alias := range aliases {
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
	if err := tmpl.Execute(&config, rewrites); err != nil {
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
	rewrites = append(rewrites, dnsTemplate(from, to, target.Query))

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
		rewrites = append(rewrites, dnsTemplate(from, to, target.Query))
	}

	return rewrites
}

func gatherRewritesForInstances(rewrites []string,
	instanceGroup bdm.InstanceGroup,
	target Target,
	namespace string,
	azIndex int,
	alias Alias) []string {
	for i := 0; i < instanceGroup.Instances; i++ {
		id := fmt.Sprintf("%s-%d", target.InstanceGroup, i)
		from := strings.Replace(alias.Domain, "_", id, 1)
		serviceName := instanceGroup.IndexedServiceName(i, azIndex)
		to := fmt.Sprintf("%s.%s.svc.%s", serviceName, namespace, clusterDomain)
		rewrites = append(rewrites, dnsTemplate(from, to, target.Query))
	}

	return rewrites
}

// The Corefile values other than the rewrites were based on the default cluster CoreDNS Corefile.
const corefileTemplate = `
.:8053 {
	errors
	health
	{{- range $rewrite := . }}
	{{ $rewrite }}
	{{- end }}
	forward . /etc/resolv.conf
	cache 30
	loop
	reload
	loadbalance
}`

func dnsTemplate(from, to, queryType string) string {
	matchPrefix := ""
	if queryType == "*" {
		matchPrefix = `(([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9\-]*[A-Za-z0-9])\.)*`
	}
	return fmt.Sprintf(cnameTemplate, regexp.QuoteMeta(from), matchPrefix, to, from)
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
