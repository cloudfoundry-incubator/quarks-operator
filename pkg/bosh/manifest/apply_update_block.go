package manifest

// ApplyUpdateBlock interprets and propagates information of the 'update'-blocks
func (m *Manifest) ApplyUpdateBlock(dns DomainNameService) {
	m.PropagateGlobalUpdateBlockToIGs()
	m.calculateRequiredServices(dns)
}

// calculateRequiredServices calculates the required services using the update.serial property
func (m *Manifest) calculateRequiredServices(dns DomainNameService) {
	var requiredService *string
	var requiredSerialService *string

	for _, ig := range m.InstanceGroups {
		serial := true
		if ig.Update != nil && ig.Update.Serial != nil {
			serial = *ig.Update.Serial
		}

		if serial {
			ig.Properties.Quarks.RequiredService = requiredService
		} else {
			ig.Properties.Quarks.RequiredService = requiredSerialService
		}

		ports := ig.ServicePorts()
		if len(ports) > 0 {
			serviceName := dns.HeadlessServiceName(ig.Name)
			requiredService = &serviceName
		}

		if serial {
			requiredSerialService = requiredService
		}
	}
}
