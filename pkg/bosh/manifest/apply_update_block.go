package manifest

// ApplyUpdateBlock interprets and propagates information of the 'update'-blocks
func (m *Manifest) ApplyUpdateBlock(dns DomainNameService) {
	m.PropagateGlobalUpdateBlockToIGs()
	m.calculateRequiredServices(dns)
}

// calculateRequiredServices calculates the required services using the update.serial property
// It follows the algorithm from BOSH:
// * it will use the last service as a dependency that had update.serial set
// * if there are no service ports, it will use the last value
func (m *Manifest) calculateRequiredServices(dns DomainNameService) {
	var requiredService *string
	var lastUsedService *string

	for _, ig := range m.InstanceGroups {
		serial := true
		if ig.Update != nil && ig.Update.Serial != nil {
			serial = *ig.Update.Serial
		}

		if serial {
			ig.Properties.Quarks.RequiredService = requiredService
		} else {
			ig.Properties.Quarks.RequiredService = lastUsedService
		}

		ports := ig.ServicePorts()
		if len(ports) > 0 {
			serviceName := dns.HeadlessServiceName(ig.Name)
			requiredService = &serviceName
		}

		if serial {
			lastUsedService = requiredService
		}
	}
}
