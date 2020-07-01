package boshdeployment

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	crc "sigs.k8s.io/controller-runtime/pkg/client"

	"code.cloudfoundry.org/quarks-operator/pkg/bosh/converter"
	bdm "code.cloudfoundry.org/quarks-operator/pkg/bosh/manifest"
	bdv1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/boshdns"
)

func matchDeployment(name string) crc.MatchingLabels {
	return map[string]string{bdv1.LabelDeploymentName: name}
}

type linkInfoService struct {
	deploymentName string
	namespace      string
}

// List returns a LinkInfos struct containing link providers if needed
// and updates `quarks_links` properties
func (l *linkInfoService) List(ctx context.Context, client crc.Client, manifest *bdm.Manifest) (converter.LinkInfos, error) {
	// find all missing providers in the manifest, so we can look for secrets
	missingProviders := manifest.ListMissingProviders()
	if len(missingProviders) == 0 {
		return converter.LinkInfos{}, nil
	}

	quarksLinks, linkInfos, err := l.nativeQuarksLinks(ctx, client, missingProviders)
	if err != nil {
		return linkInfos, err
	}

	if len(quarksLinks) != 0 {
		if manifest.Properties == nil {
			manifest.Properties = map[string]interface{}{}
		}
		manifest.Properties[bdm.QuarksLinksProperty] = quarksLinks
	}
	return linkInfos, err
}

// nativeQuarksLinks finds secrets for all missing links. It creates the link
// properties and uses data from existing services.
func (l *linkInfoService) nativeQuarksLinks(ctx context.Context, client crc.Client, missingProviders map[string]bool) (map[string]bdm.QuarksLink, converter.LinkInfos, error) {
	linkInfos := converter.LinkInfos{}
	// quarksLinks store for missingProvider names with types read from secrets
	quarksLinks := map[string]bdm.QuarksLink{}

	// list secrets and services from target deployment
	secrets := &corev1.SecretList{}
	err := client.List(ctx, secrets,
		crc.InNamespace(l.namespace),
		matchDeployment(l.deploymentName),
	)
	if err != nil {
		return quarksLinks, linkInfos, errors.Wrap(err, "listing secrets to fill missing links")
	}

	// Create linkInfos/quarksLinks for all missingProviders, which are provided by secrets. Error out if
	// two secrets exist for the same link.
	for _, s := range secrets.Items {
		if !isLinkProviderSecret(s) {
			continue
		}
		linkProvider, err := newLinkProvider(s.GetAnnotations())
		if err != nil {
			return quarksLinks, linkInfos, errors.Wrapf(err, "failed to parse link annotation JSON for secret '%s'", s.Name)
		}
		if dup, ok := missingProviders[linkProvider.Name]; ok {
			if dup {
				return quarksLinks, linkInfos, errors.New(fmt.Sprintf("duplicated secrets of provider: %s", linkProvider.Name))
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

	// Check if all missingProviders are now backed by secrets, otherwise throw an error
	missingPs := make([]string, 0, len(missingProviders))
	for key, found := range missingProviders {
		if !found {
			missingPs = append(missingPs, key)
		}
	}

	if len(missingPs) != 0 {
		return quarksLinks, linkInfos, errors.New(fmt.Sprintf("missing link secrets for providers: %s", strings.Join(missingPs, ", ")))
	}

	// Find services that are relevant to the new QuarksLinks
	services := &corev1.ServiceList{}
	err = client.List(ctx, services,
		crc.InNamespace(l.namespace),
		matchDeployment(l.deploymentName),
	)
	if err != nil {
		return quarksLinks, linkInfos, errors.Wrap(err, "listing services")
	}

	serviceRecords, err := serviceRecordByProvider(ctx, client, l.namespace, linkedServices(services.Items))
	if err != nil {
		return quarksLinks, linkInfos, errors.Wrap(err, "failed to determine service records of link providers")
	}

	// Update quarksLinks section `manifest.Properties["quarks_links"]` with info from existing serviceRecords
	for qName := range quarksLinks {
		if svcRecord, ok := serviceRecords[qName]; ok {
			j, err := svcRecord.jobInstances(ctx, client, l.namespace, qName)
			if err != nil {
				return quarksLinks, linkInfos, errors.Wrapf(err, "failed to get job instances for service record '%s'", qName)
			}
			quarksLinks[qName] = bdm.QuarksLink{
				Type:      quarksLinks[qName].Type,
				Address:   svcRecord.dnsRecord,
				Instances: j,
			}
		}
	}

	return quarksLinks, linkInfos, nil
}

func linkedServices(services []corev1.Service) []corev1.Service {
	filtered := make([]corev1.Service, 0, len(services))
	for _, svc := range services {
		if isLinkProviderService(&svc) {
			filtered = append(filtered, svc)
		}

	}
	return filtered
}

// serviceRecordByProvider creates a map of service records from the linked k8s services
// the map contains one serviceRecord per link provider
// For every service with a link provider annotation, we look create a service record struct:
// * ServiceTypeExternalName: no addresses, no selector
// * with selector: addresses have to be filled from selected pods
// * otherwise look for an endpoint, addresses have to be filled from the endpoint
// * error otherwise
func serviceRecordByProvider(ctx context.Context, client crc.Client, namespace string, services []corev1.Service) (map[string]serviceRecord, error) {
	svcRecords := map[string]serviceRecord{}
	for _, svc := range services {
		providerName := svc.GetAnnotations()[bdv1.AnnotationLinkProviderName]
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

		// Selector can be used to gather data from pods
		if len(svc.Spec.Selector) != 0 {
			svcRecords[providerName] = serviceRecord{
				addresses: nil,
				selector:  svc.Spec.Selector,
				dnsRecord: fmt.Sprintf("%s.%s.svc.%s", svc.Name, namespace, boshdns.GetClusterDomain()),
			}

			continue
		}

		// If we don't have a selector, the service must be backed by a manually created Endpoint with the same name.
		endpoints := &corev1.Endpoints{}
		if err := client.Get(ctx, types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}, endpoints); err != nil {
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

// jobInstances returns quarks link job instances from the service record
// this will fill the name, id, index, address and bootstrap fields for each instance of a job from the BOSH manifest.
// This func tries the following in the given order:
// * selector is present, use information from matched pods
// * if addresses are present use those
// * use the service's DNS address as a fallback
func (sr serviceRecord) jobInstances(ctx context.Context, client crc.Client, namespace string, qName string) ([]bdm.JobInstance, error) {
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
