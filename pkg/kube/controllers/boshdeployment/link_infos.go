package boshdeployment

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	crc "sigs.k8s.io/controller-runtime/pkg/client"

	"code.cloudfoundry.org/quarks-operator/pkg/bosh/converter"
	bdm "code.cloudfoundry.org/quarks-operator/pkg/bosh/manifest"
	bdv1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/boshdns"
)

// ListLinkInfos returns a LinkInfos containing link providers if needed
// and updates `quarks_links` properties
func ListLinkInfos(ctx context.Context, client crc.Client, bdpl *bdv1.BOSHDeployment, manifest *bdm.Manifest) (converter.LinkInfos, error) {
	linkInfos := converter.LinkInfos{}

	// find all missing providers in the manifest, so we can look for secrets
	missingProviders := manifest.ListMissingProviders()

	// quarksLinks store for missing provider names with types read from secrets
	quarksLinks := map[string]bdm.QuarksLink{}
	if len(missingProviders) != 0 {
		// list secrets and services from target deployment
		secrets := &corev1.SecretList{}
		err := client.List(ctx, secrets,
			crc.InNamespace(bdpl.Namespace),
		)
		if err != nil {
			return linkInfos, errors.Wrapf(err, "listing secrets for link in deployment '%s':", bdpl.GetNamespacedName())
		}

		for _, s := range secrets.Items {
			if deploymentName, ok := s.GetAnnotations()[bdv1.LabelDeploymentName]; !ok || deploymentName != bdpl.Name {
				continue
			}

			linkProvider, err := newLinkProvider(s.GetAnnotations())
			if err != nil {
				return linkInfos, errors.Wrapf(err, "failed to parse link JSON for '%s'", bdpl.GetNamespacedName())
			}
			if dup, ok := missingProviders[linkProvider.Name]; ok {
				if dup {
					return linkInfos, errors.New(fmt.Sprintf("duplicated secrets of provider: %s", linkProvider.Name))
				}

				linkInfos = append(linkInfos, converter.LinkInfo{
					SecretName:   s.Name,
					ProviderName: linkProvider.Name,
					ProviderType: linkProvider.ProviderType,
				})

				if linkProvider.ProviderType != "" {
					quarksLinks[linkProvider.Name] = bdm.QuarksLink{
						Type: linkProvider.ProviderType,
					}
				}
				missingProviders[linkProvider.Name] = true
			}
		}

		services := &corev1.ServiceList{}
		err = client.List(ctx, services,
			crc.InNamespace(bdpl.Namespace),
		)
		if err != nil {
			return linkInfos, errors.Wrapf(err, "listing services for link in deployment '%s':", bdpl.GetNamespacedName())
		}

		serviceRecords, err := serviceRecordByProvider(ctx, client, bdpl.Namespace, linkedServices(bdpl.Name, services.Items))
		if err != nil {
			return linkInfos, errors.Wrapf(err, "failed to get link services for '%s'", bdpl.GetNamespacedName())
		}

		// Update quarksLinks section `manifest.Properties["quarks_links"]` with info from existing serviceRecords
		for qName := range quarksLinks {
			if svcRecord, ok := serviceRecords[qName]; ok {
				j, err := svcRecord.JobInstances(ctx, client, bdpl.Namespace, qName)
				if err != nil {
					return linkInfos, errors.Wrapf(err, "failed to get job instances from service record for '%s'", bdpl.GetNamespacedName())
				}
				quarksLinks[qName] = bdm.QuarksLink{
					Type:      quarksLinks[qName].Type,
					Address:   svcRecord.dnsRecord,
					Instances: j,
				}
			}
		}
	}

	missingPs := make([]string, 0, len(missingProviders))
	for key, found := range missingProviders {
		if !found {
			missingPs = append(missingPs, key)
		}
	}

	if len(missingPs) != 0 {
		return linkInfos, errors.New(fmt.Sprintf("missing link secrets for providers: %s", strings.Join(missingPs, ", ")))
	}

	if len(quarksLinks) != 0 {
		if manifest.Properties == nil {
			manifest.Properties = map[string]interface{}{}
		}
		manifest.Properties[bdm.QuarksLinksProperty] = quarksLinks
	}

	return linkInfos, nil
}

func linkedServices(name string, services []corev1.Service) []corev1.Service {
	filtered := make([]corev1.Service, len(services))
	for _, svc := range services {
		if deploymentName, ok := svc.GetAnnotations()[bdv1.LabelDeploymentName]; !ok || deploymentName != name {
			continue
		}

		if _, ok := svc.GetAnnotations()[bdv1.AnnotationLinkProviderService]; !ok {
			continue
		}
		filtered = append(filtered, svc)

	}
	return filtered
}

// serviceRecordByProvider creates a map of service records from the linked k8s services
// the map contains one serviceRecord per link provider
func serviceRecordByProvider(ctx context.Context, client crc.Client, namespace string, services []corev1.Service) (map[string]serviceRecord, error) {
	svcRecords := map[string]serviceRecord{}
	for _, svc := range services {
		providerName := svc.GetAnnotations()[bdv1.AnnotationLinkProviderService]
		if _, ok := svcRecords[providerName]; ok {
			return svcRecords, errors.New(fmt.Sprintf("duplicated services of provider: %s", providerName))
		}

		// An ExternalName service doesn't have a selector or endpoints
		if svc.Spec.Type == corev1.ServiceTypeExternalName {
			svcRecords[providerName] = serviceRecord{
				addresses: nil,
				selector:  nil,
				dnsRecord: fmt.Sprintf("%s.%s.svc.%s", svc.Name, namespace, boshdns.GetClusterDomain()),
			}

			continue
		}

		if len(svc.Spec.Selector) != 0 {
			svcRecords[providerName] = serviceRecord{
				addresses: nil,
				selector:  svc.Spec.Selector,
				dnsRecord: fmt.Sprintf("%s.%s.svc.%s", svc.Name, namespace, boshdns.GetClusterDomain()),
			}

			continue
		}

		// If we don't have a selector, we're either dealing with an ExternalName service,
		// or a service that's backed by manually created Endpoints.
		endpoints := &corev1.Endpoints{}
		err := client.Get(
			ctx,
			types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace},
			endpoints)

		if err != nil {
			// No selectors and no endpoints
			if apierrors.IsNotFound(err) {
				svcRecords[providerName] = serviceRecord{
					addresses: nil,
					selector:  nil,
					dnsRecord: fmt.Sprintf("%s.%s.svc.%s", svc.Name, namespace, boshdns.GetClusterDomain()),
				}
			}

			// We hit an actual error
			return nil, errors.Wrapf(err, "failed to get service endpoints for links")
		}

		addresses := []string{}
		for _, subset := range endpoints.Subsets {
			for _, address := range subset.Addresses {
				addresses = append(addresses, address.IP)
			}
		}

		svcRecords[providerName] = serviceRecord{
			addresses: addresses,
			selector:  nil,
			dnsRecord: fmt.Sprintf("%s.%s.svc.%s", svc.Name, namespace, boshdns.GetClusterDomain()),
		}
	}

	return svcRecords, nil
}

type serviceRecord struct {
	selector  map[string]string
	dnsRecord string
	addresses []string
}

// JobInstances returns quarks link job instances from the service record
func (sr serviceRecord) JobInstances(ctx context.Context, client crc.Client, namespace string, qName string) ([]bdm.JobInstance, error) {
	var jobsInstances []bdm.JobInstance

	if sr.selector != nil {
		// Service has selectors, we're going through pods in order to build
		// an instance list for the link
		pods, err := listPodsByLabel(ctx, client, namespace, sr.selector)
		if err != nil {
			return jobsInstances, errors.Wrapf(err, "Failed to get link pods by label")
		}

		for i, p := range pods {
			if len(p.Status.PodIP) == 0 {
				return jobsInstances, fmt.Errorf("empty ip of kube native component: '%s'", p.Name)
			}
			jobsInstances = append(jobsInstances, bdm.JobInstance{
				Name:      qName,
				ID:        string(p.GetUID()),
				Index:     i,
				Address:   p.Status.PodIP,
				Bootstrap: i == 0,
			})
		}
	} else if sr.addresses != nil {
		for i, a := range sr.addresses {
			jobsInstances = append(jobsInstances, bdm.JobInstance{
				Name:      qName,
				ID:        a,
				Index:     i,
				Address:   a,
				Bootstrap: i == 0,
			})
		}
	} else {
		// No selector, no addresses - we're creating one instance that just points to the service address itself
		jobsInstances = append(jobsInstances, bdm.JobInstance{
			Name:      qName,
			ID:        qName,
			Index:     0,
			Address:   sr.dnsRecord,
			Bootstrap: true,
		})
	}

	return jobsInstances, nil
}

// listPodsByLabel returns a list of pods matching the labels in selector
func listPodsByLabel(ctx context.Context, client crc.Client, namespace string, selector map[string]string) ([]corev1.Pod, error) {
	podList := &corev1.PodList{}
	err := client.List(ctx, podList,
		crc.InNamespace(namespace),
		crc.MatchingLabels(selector),
	)
	if err != nil {
		return podList.Items, errors.Wrapf(err, "listing pods from selector '%+v':", selector)
	}

	return podList.Items, nil
}
