package openstack

// func (provider ProviderOpenstack) Deploy(ctx context.Context, request *providerGRPC.DeployRequest) (*providerGRPC.DeployReply, error) {
// 	logrus.Debugf("Deploy called for deployment \"%s\"", request.DeploymentId)

// 	// Check if the provider has been configured
// 	if CONFIG == nil {
// 		return nil, fmt.Errorf("cannot deploy with unconfigured provider, please call Configure()")
// 	}

// 	// Generate authenticated client session
// 	authClient, err := provider.newAuthClient()
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to authenticate: %v", err)
// 	}

// 	// Parse blueprint into struct
// 	blueprint, err := UnmarshalBlueprintBytesWithVars(request.Blueprint, request.TemplateVars.AsMap())
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to unmarshal blueprint: %v", err)
// 	}

// 	// Validate the blueprint is valid
// 	err = ValidateBlueprint(blueprint)
// 	if err != nil {
// 		return nil, fmt.Errorf("blueprint is invalid: %v", err)
// 	}

// 	// Generate local routine-safe var map
// 	var varMap sync.Map
// 	for k, v := range request.DeploymentVars.AsMap() {
// 		varMap.Store(k, v)
// 	}
// 	// Generate local routine-safe state map
// 	var stateMap sync.Map
// 	for k, v := range request.DeploymentState.AsMap() {
// 		stateMap.Store(k, v)
// 	}

// 	response := providerGRPC.DeployReply{
// 		DeploymentId: request.DeploymentId,
// 		Status:       commonGRPC.RPCStatus_SUCCESS,
// 		Errors:       []string{},
// 	}

// 	objectsWg := sync.WaitGroup{}
// 	for k := range blueprint.Objects {
// 		objectsWg.Add(1)
// 		go func(key string) {
// 			// Wait until all depends_on are done
// 			err := awaitDependsOn(blueprint, &stateMap, key)
// 			if err != nil {
// 				logrus.Errorf("failed to deploy network: %v", err)
// 			} else {
// 				switch blueprint.Objects[key].Resource {
// 				// HOST
// 				case OpenstackResourceTypeHost:
// 					// Deploy host
// 					if err := provider.deployHost(ctx, authClient, request.DeploymentId, &varMap, &stateMap, blueprint, key); err != nil {
// 						response.Status = commonGRPC.RPCStatus_FAILURE
// 						response.Errors = append(response.Errors, fmt.Sprintf("failed to deploy host \"%s\": %v", key, err))
// 					}
// 				// NETWORK
// 				case OpenstackResourceTypeNetwork:
// 					// Deploy network
// 					if err := provider.deployNetwork(ctx, authClient, request.DeploymentId, &varMap, &stateMap, blueprint, key); err != nil {
// 						response.Status = commonGRPC.RPCStatus_FAILURE
// 						response.Errors = append(response.Errors, fmt.Sprintf("failed to deploy host \"%s\": %v", key, err))
// 					}
// 				// ROUTER
// 				case OpenstackResourceTypeRouter:
// 					// Deploy router
// 					if err := provider.deployRouter(ctx, authClient, request.DeploymentId, &varMap, &stateMap, blueprint, key); err != nil {
// 						response.Status = commonGRPC.RPCStatus_FAILURE
// 						response.Errors = append(response.Errors, fmt.Sprintf("failed to deploy host \"%s\": %v", key, err))
// 					}
// 				}
// 			}
// 			objectsWg.Done()
// 		}(k)
// 	}
// 	objectsWg.Wait()

// 	response.DeploymentState, err = marshalSyncMap(&stateMap)
// 	if err != nil {
// 		response.Errors = append(response.Errors, fmt.Sprintf("failed to marshal stateMap: %v", err))
// 	}
// 	response.DeploymentVars, err = marshalSyncMap(&varMap)
// 	if err != nil {
// 		response.Errors = append(response.Errors, fmt.Sprintf("failed to marshal varMap: %v", err))
// 	}

// 	return &response, nil
// }

// func (provider *ProviderOpenstack) deployNetwork(ctx context.Context, authClient *gophercloud.ProviderClient, deploymentId string, varMap *sync.Map, stateMap *sync.Map, blueprint *OpenstackBlueprint, networkKey string) error {
// 	logrus.Debugf("Deploying network \"%s\"", networkKey)

// 	var err error
// 	// Get the network from blueprint
// 	network, exist := blueprint.Networks[networkKey]
// 	if !exist {
// 		stateMap.Store(networkKey, commonGRPC.DeployStateFAILED)
// 		return fmt.Errorf("network \"%s\" is not defined", networkKey)
// 	}

// 	// Set network as in progress for dependencies
// 	stateMap.Store(networkKey, commonGRPC.DeployStateINPROGRESS)

// 	// Generate the Network V2 client
// 	endpointOpts := gophercloud.EndpointOpts{
// 		Name:   "neutron",
// 		Region: CONFIG.RegionName,
// 	}
// 	networkClient, err := openstack.NewNetworkV2(authClient, endpointOpts)
// 	if err != nil {
// 		stateMap.Store(networkKey, commonGRPC.DeployStateFAILED)
// 		return fmt.Errorf("failed to create openstack network client: %v", err)
// 	}

// 	networkName := networkKey
// 	if network.Name != nil {
// 		networkName = *network.Name
// 	}
// 	// Prepend the first 8 bytes of deployment ID
// 	networkName = deploymentId[:8] + "-" + networkName

// 	// Create the network
// 	deployedNetwork, err := networks.Create(networkClient, networks.CreateOpts{
// 		Name:         networkName,
// 		AdminStateUp: gophercloud.Enabled,
// 	}).Extract()
// 	if err != nil {
// 		stateMap.Store(networkKey, commonGRPC.DeployStateFAILED)
// 		return fmt.Errorf("failed to create network: %v", err)
// 	}

// 	// Save the deployed network id into vars
// 	varMap.Store(networkKey+"_id", deployedNetwork.ID)

// 	// Configure the subnet on the network
// 	var gatewayIp *string = nil
// 	if network.Gateway != nil {
// 		gatewayString := network.Gateway.String()
// 		gatewayIp = &gatewayString
// 	}
// 	dhcpPools := []subnets.AllocationPool{}
// 	for _, dhcp := range network.DHCP {
// 		dhcpPools = append(dhcpPools, subnets.AllocationPool{
// 			Start: dhcp.Start.String(),
// 			End:   dhcp.End.String(),
// 		})
// 	}
// 	dnsServers := []string{}
// 	for _, resolverIP := range network.Resolvers {
// 		dnsServers = append(dnsServers, resolverIP.String())
// 	}

// 	// Create openstack subnet on network
// 	deployedSubnet, err := subnets.Create(networkClient, subnets.CreateOpts{
// 		NetworkID:       deployedNetwork.ID,
// 		CIDR:            network.Subnet.String(),
// 		Name:            networkName,
// 		Description:     fmt.Sprintf("%s Subnet for Network \"%s\"", network.Subnet.String(), networkName),
// 		AllocationPools: dhcpPools,
// 		GatewayIP:       gatewayIp,
// 		IPVersion:       gophercloud.IPv4,
// 		EnableDHCP:      gophercloud.Enabled,
// 		DNSNameservers:  dnsServers,
// 	}).Extract()
// 	if err != nil {
// 		stateMap.Store(networkKey, commonGRPC.DeployStateFAILED)
// 		return fmt.Errorf("failed to create subnet: %v", err)
// 	}

// 	// Save the deployed network subnet id into vars
// 	varMap.Store(networkKey+"_subnet_id", deployedSubnet.ID)

// 	logrus.Debugf("Successfully deployed network %s as network %s (%s)", networkKey, deployedNetwork.Name, deployedNetwork.ID)

// 	// Set network as succeeded for dependencies
// 	stateMap.Store(networkKey, commonGRPC.DeployStateSUCCEEDED)

// 	return nil
// }

// func (provider *ProviderOpenstack) deployRouter(ctx context.Context, authClient *gophercloud.ProviderClient, deploymentId string, varMap *sync.Map, stateMap *sync.Map, blueprint *OpenstackBlueprint, routerKey string) error {
// 	logrus.Debugf("Deploying router \"%s\"", routerKey)

// 	var err error
// 	// Get the router from blueprint
// 	router, exist := blueprint.Routers[routerKey]
// 	if !exist {
// 		stateMap.Store(routerKey, commonGRPC.DeployStateFAILED)
// 		return fmt.Errorf("router \"%s\" is not defined", routerKey)
// 	}

// 	// Set router as in progress for dependencies
// 	stateMap.Store(routerKey, commonGRPC.DeployStateINPROGRESS)

// 	// Generate the Network V2 client
// 	endpointOpts := gophercloud.EndpointOpts{
// 		Name:   "neutron",
// 		Region: CONFIG.RegionName,
// 	}
// 	networkClient, err := openstack.NewNetworkV2(authClient, endpointOpts)
// 	if err != nil {
// 		stateMap.Store(routerKey, commonGRPC.DeployStateFAILED)
// 		return fmt.Errorf("failed to create openstack network client: %v", err)
// 	}

// 	// Find the external network
// 	var routerExternalNetwork *networks.Network = nil
// 	allNetworkPages, err := networks.List(networkClient, nil).AllPages()
// 	if err != nil {
// 		stateMap.Store(routerKey, commonGRPC.DeployStateFAILED)
// 		return fmt.Errorf("failed to get router external network \"%s\": %v", router.ExternalNetwork, err)
// 	}
// 	allNetworks, err := networks.ExtractNetworks(allNetworkPages)
// 	if err != nil {
// 		stateMap.Store(routerKey, commonGRPC.DeployStateFAILED)
// 		return fmt.Errorf("failed to get router external network \"%s\": %v", router.ExternalNetwork, err)
// 	}
// 	for _, net := range allNetworks {
// 		if net.Name == router.ExternalNetwork || net.ID == router.ExternalNetwork {
// 			routerExternalNetwork = &net
// 			break
// 		}
// 	}
// 	if routerExternalNetwork == nil {
// 		stateMap.Store(routerKey, commonGRPC.DeployStateFAILED)
// 		return fmt.Errorf("failed to get router external network \"%s\": network not found", router.ExternalNetwork)
// 	}

// 	// Prepend the first 8 bytes of deployment ID
// 	routerName := deploymentId[:8] + "-" + routerKey

// 	routerConfig := routers.CreateOpts{
// 		Name:         routerName,
// 		Description:  "",
// 		AdminStateUp: gophercloud.Enabled,
// 		GatewayInfo: &routers.GatewayInfo{
// 			NetworkID: routerExternalNetwork.ID,
// 		},
// 	}
// 	if router.Name != nil {
// 		routerConfig.Name = *router.Name
// 	}
// 	if router.Description != nil {
// 		routerConfig.Description = *router.Description
// 	}

// 	// Deploy the router
// 	deployedRouter, err := routers.Create(networkClient, routerConfig).Extract()
// 	if err != nil {
// 		stateMap.Store(routerKey, commonGRPC.DeployStateFAILED)
// 		return fmt.Errorf("failed to create router: %v", err)
// 	}

// 	// Save the deployed router into vars
// 	varMap.Store(routerKey+"_id", deployedRouter.ID)

// 	// Connect router to all attached networks

// 	for k, networkAttachment := range router.Networks {
// 		networkId, exists := varMap.Load(k + "_id")
// 		if !exists {
// 			stateMap.Store(routerKey, commonGRPC.DeployStateFAILED)
// 			return fmt.Errorf("ID unknown for network \"%s\"", k)
// 		}
// 		networkSubnetId, exists := varMap.Load(k + "_subnet_id")
// 		if !exists {
// 			stateMap.Store(routerKey, commonGRPC.DeployStateFAILED)
// 			return fmt.Errorf("ID unknown for network \"%s\" subnet", k)
// 		}
// 		// Create Openstack port for router on subnet
// 		osPort, err := ports.Create(networkClient, ports.CreateOpts{
// 			NetworkID:    networkId.(string),
// 			AdminStateUp: gophercloud.Enabled,
// 			FixedIPs: []ports.IP{{
// 				SubnetID:  networkSubnetId.(string),
// 				IPAddress: networkAttachment.IP.String(),
// 			}},
// 		}).Extract()
// 		if err != nil {
// 			stateMap.Store(routerKey, commonGRPC.DeployStateFAILED)
// 			return fmt.Errorf("failed to create port for router: %v", err)
// 		}

// 		// Save the deployed router network port into vars
// 		varMap.Store(routerKey+"_"+k+"_port_id", osPort.ID)

// 		// We don't need to store this ID since it will get auto-deleted on router delete
// 		_, err = routers.AddInterface(networkClient, deployedRouter.ID, routers.AddInterfaceOpts{
// 			PortID: osPort.ID,
// 		}).Extract()
// 		if err != nil {
// 			stateMap.Store(routerKey, commonGRPC.DeployStateFAILED)
// 			return fmt.Errorf("failed to create router interface: %v", err)
// 		}
// 	}

// 	logrus.Debugf("Successfully deployed router %s as router %s (%s)", routerKey, deployedRouter.Name, deployedRouter.ID)

// 	// Set router as succeeded for dependencies
// 	stateMap.Store(routerKey, commonGRPC.DeployStateSUCCEEDED)

// 	return nil
// }

// func (provider *ProviderOpenstack) deployHost(ctx context.Context, authClient *gophercloud.ProviderClient, deploymentId string, varMap *sync.Map, stateMap *sync.Map, blueprint *OpenstackBlueprint, hostKey string) error {
// 	logrus.Debugf("Deploying host \"%s\"", hostKey)

// 	var err error
// 	// Get the host from blueprint
// 	host, exist := blueprint.Hosts[hostKey]
// 	if !exist {
// 		stateMap.Store(hostKey, commonGRPC.DeployStateFAILED)
// 		return fmt.Errorf("host \"%s\" is not defined", hostKey)
// 	}

// 	// Set host as in progress for dependencies
// 	stateMap.Store(hostKey, commonGRPC.DeployStateINPROGRESS)

// 	// Generate the Compute V2 client
// 	endpointOpts := gophercloud.EndpointOpts{
// 		Region: CONFIG.RegionName,
// 	}
// 	computeClient, err := openstack.NewComputeV2(authClient, endpointOpts)
// 	if err != nil {
// 		stateMap.Store(hostKey, commonGRPC.DeployStateFAILED)
// 		return fmt.Errorf("failed to create compute v2 client: %v", err)
// 	}

// 	var hostFlavor *flavors.Flavor = nil
// 	allFlavorPages, err := flavors.ListDetail(computeClient, nil).AllPages()
// 	if err != nil {
// 		stateMap.Store(hostKey, commonGRPC.DeployStateFAILED)
// 		return fmt.Errorf("failed to get host flavor \"%s\": %v", host.Flavor, err)
// 	}
// 	allFlavors, err := flavors.ExtractFlavors(allFlavorPages)
// 	if err != nil {
// 		stateMap.Store(hostKey, commonGRPC.DeployStateFAILED)
// 		return fmt.Errorf("failed to get host flavor \"%s\": %v", host.Flavor, err)
// 	}
// 	for _, fl := range allFlavors {
// 		if fl.Name == host.Flavor || fl.ID == host.Flavor {
// 			hostFlavor = &fl
// 			break
// 		}
// 	}
// 	if hostFlavor == nil {
// 		stateMap.Store(hostKey, commonGRPC.DeployStateFAILED)
// 		return fmt.Errorf("failed to get host flavor \"%s\": flavor not found", host.Flavor)
// 	}

// 	logrus.Debugf("got flavor %s (%s)", hostFlavor.Name, hostFlavor.ID)

// 	var hostImage *images.Image = nil
// 	allImagePages, err := images.ListDetail(computeClient, nil).AllPages()
// 	if err != nil {
// 		stateMap.Store(hostKey, commonGRPC.DeployStateFAILED)
// 		return fmt.Errorf("failed to get host image \"%s\": %v", host.Image, err)
// 	}
// 	allImages, err := images.ExtractImages(allImagePages)
// 	if err != nil {
// 		stateMap.Store(hostKey, commonGRPC.DeployStateFAILED)
// 		return fmt.Errorf("failed to get host image \"%s\": %v", host.Image, err)
// 	}
// 	for _, img := range allImages {
// 		if img.Name == host.Image || img.ID == host.Image {
// 			hostImage = &img
// 			break
// 		}
// 	}
// 	if hostImage == nil {
// 		stateMap.Store(hostKey, commonGRPC.DeployStateFAILED)
// 		return fmt.Errorf("failed to get host image \"%s\": image not found", host.Image)
// 	}

// 	// Check if the image requires more space than provided
// 	if host.DiskSize < hostImage.MinDisk {
// 		stateMap.Store(hostKey, commonGRPC.DeployStateFAILED)
// 		return fmt.Errorf("host disk size is too small for image (minimum %dGB required)", hostImage.MinDisk)
// 	}

// 	logrus.Debugf("got image %s (%s)", hostImage.Name, hostImage.ID)

// 	// Use either key or provided name as instance name
// 	instanceName := host.Hostname
// 	if host.Name != nil {
// 		instanceName = *host.Name
// 	}
// 	// Prepend the first 8 bytes of deployment ID
// 	instanceName = deploymentId[:8] + "-" + instanceName

// 	// Configure the volume to clone from the image
// 	blockOps := []bootfromvolume.BlockDevice{
// 		{
// 			UUID:                hostImage.ID,
// 			BootIndex:           0,
// 			DeleteOnTermination: true,
// 			DestinationType:     bootfromvolume.DestinationVolume,
// 			SourceType:          bootfromvolume.SourceImage,
// 			VolumeSize:          host.DiskSize,
// 		},
// 	}

// 	// Create network mappings for each network attachment
// 	hostNetworks := []servers.Network{}
// 	for k, networkAttachment := range host.Networks {
// 		_, exists := blueprint.Networks[k]
// 		if !exists {
// 			stateMap.Store(hostKey, commonGRPC.DeployStateFAILED)
// 			return fmt.Errorf("network \"%s\" is not defined", k)
// 		}
// 		networkId, exists := varMap.Load(k + "_id")
// 		if !exists {
// 			stateMap.Store(hostKey, commonGRPC.DeployStateFAILED)
// 			return fmt.Errorf("ID unknown for network \"%s\"", k)
// 		}
// 		PLACEHOLDER_NET := servers.Network{
// 			UUID: networkId.(string),
// 		}
// 		if !networkAttachment.DHCP && networkAttachment.IP != nil {
// 			PLACEHOLDER_NET.FixedIP = networkAttachment.IP.String()
// 		}
// 		hostNetworks = append(hostNetworks, PLACEHOLDER_NET)
// 	}

// 	// Configure the instance options
// 	hostOps := servers.CreateOpts{
// 		Name:      instanceName,
// 		ImageRef:  hostImage.ID,
// 		FlavorRef: hostFlavor.ID,
// 		UserData:  host.UserData,
// 		Networks:  hostNetworks,
// 	}

// 	// Create the host
// 	createOpts := bootfromvolume.CreateOptsExt{
// 		CreateOptsBuilder: hostOps,
// 		BlockDevice:       blockOps,
// 	}
// 	deployedServer, err := bootfromvolume.Create(computeClient, createOpts).Extract()
// 	if err != nil {
// 		stateMap.Store(hostKey, commonGRPC.DeployStateFAILED)
// 		return fmt.Errorf("failed to deploy host: %v", err)
// 	}

// 	// Save the deployed host into vars
// 	varMap.Store(hostKey+"_id", deployedServer.ID)

// 	// Wait for server to be in ACTIVE state
// 	for {
// 		// Get the updated server from Openstack
// 		deployedServer, err = servers.Get(computeClient, deployedServer.ID).Extract()
// 		if err != nil {
// 			stateMap.Store(hostKey, commonGRPC.DeployStateFAILED)
// 			return fmt.Errorf("failed to get openstack server status: %v", err)
// 		}
// 		if deployedServer.Status == "ERROR" {
// 			// Something happened and this failed
// 			stateMap.Store(hostKey, commonGRPC.DeployStateFAILED)
// 			return fmt.Errorf("failed to deploy host: server in ERROR state")
// 		}
// 		if deployedServer.Status == "ACTIVE" {
// 			// Server deployed properly
// 			break
// 		}
// 		// Wait 5 seconds before checking again
// 		time.Sleep(5 * time.Second)
// 	}

// 	logrus.Debugf("Successfully deployed host %s as server %s (%s)", hostKey, deployedServer.Name, deployedServer.ID)

// 	// Set host as succeeded for dependencies
// 	stateMap.Store(hostKey, commonGRPC.DeployStateSUCCEEDED)

// 	return nil
// }

// // Blocks execution until all depends_on are done.
// func awaitDependsOn(blueprint *OpenstackBlueprint, stateMap *sync.Map, key string) error {
// 	// Check on dependencies
// 	for {
// 		waitingOnDependents := false
// 		for _, dependsOnKey := range blueprint.Objects[key].DependsOn {
// 			dependsOnDeploymentValue, exists := stateMap.Load(dependsOnKey)
// 			if exists {
// 				dependsOnDeploymentState := dependsOnDeploymentValue.(string)
// 				if dependsOnDeploymentState == commonGRPC.DeployStateFAILED {
// 					return fmt.Errorf("\"%s\" dependency \"%s\" failed", key, dependsOnKey)
// 				} else if dependsOnDeploymentState == commonGRPC.DeployStateSUCCEEDED {
// 					// Dependent is deployed so we're good
// 					continue
// 				} else {
// 					logrus.Debugf("\"%s\" is waiting on \"%s\"", key, dependsOnKey)
// 					// early break since no need to check others if a single dependency is still inactive
// 					waitingOnDependents = true
// 					break
// 				}
// 			} else {
// 				waitingOnDependents = true
// 			}
// 		}
// 		// If all depends on objects are done
// 		if !waitingOnDependents {
// 			break
// 		}
// 		// Wait 5 secs before checking again
// 		time.Sleep(5 * time.Second)
// 	}
// 	return nil
// }
