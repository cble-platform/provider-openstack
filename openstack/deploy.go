package openstack

import (
	"context"
	"fmt"
	"time"

	providerGRPC "github.com/cble-platform/cble-provider-grpc/pkg/provider"
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/bootfromvolume"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/flavors"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/images"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/layer3/routers"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/ports"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/subnets"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

func (provider ProviderOpenstack) DeployResource(ctx context.Context, request *providerGRPC.DeployResourceRequest) (*providerGRPC.DeployResourceReply, error) {
	logrus.Debugf("----- DeployResource called for deployment (%s) resource %s -----", request.Deployment.Id, request.Resource.Key)

	// Check if the provider has been configured
	if CONFIG == nil {
		return &providerGRPC.DeployResourceReply{
			Success: false,
			Errors:  Errorf("cannot deploy with unconfigured provider, please call Configure()"),
		}, nil
	}

	// Generate authenticated client session
	authClient, err := provider.newAuthClient()
	if err != nil {
		return &providerGRPC.DeployResourceReply{
			Success: false,
			Errors:  Errorf("failed to authenticate: %v", err),
		}, nil
	}

	// Unmarshal the object YAML as struct
	var object *OpenstackObject
	err = yaml.Unmarshal(request.Resource.Object, &object)
	if err != nil {
		return &providerGRPC.DeployResourceReply{
			Success: false,
			Errors:  Errorf("failed to unmarshal resource object: %v", err),
		}, nil
	}

	// Check this is a resource (not data)
	if object.Resource == nil {
		return &providerGRPC.DeployResourceReply{
			Success: false,
			Errors:  Errorf("cannot deploy data object"),
		}, nil
	}

	var updatedVars map[string]string

	// Deploy the resource based on the type of resource
	switch *object.Resource {
	// HOST
	case OpenstackResourceTypeHost:
		// Deploy host
		if updatedVars, err = provider.deployHost(ctx, authClient, request, object, request.Vars, request.DependencyVars); err != nil {
			return &providerGRPC.DeployResourceReply{
				Success: false,
				Errors:  Errorf("failed to deploy host: %v", err),
			}, nil
		}
	// NETWORK
	case OpenstackResourceTypeNetwork:
		// Deploy network
		if updatedVars, err = provider.deployNetwork(ctx, authClient, request, object, request.Vars, request.DependencyVars); err != nil {
			return &providerGRPC.DeployResourceReply{
				Success: false,
				Errors:  Errorf("failed to deploy network: %v", err),
			}, nil
		}
	// ROUTER
	case OpenstackResourceTypeRouter:
		// Deploy router
		if updatedVars, err = provider.deployRouter(ctx, authClient, request, object, request.Vars, request.DependencyVars); err != nil {
			return &providerGRPC.DeployResourceReply{
				Success: false,
				Errors:  Errorf("failed to deploy router: %v", err),
			}, nil
		}
	}

	// Return the updated vars
	return &providerGRPC.DeployResourceReply{
		Success:     true,
		Errors:      nil,
		UpdatedVars: updatedVars,
	}, nil
}

func (provider *ProviderOpenstack) deployHost(ctx context.Context, authClient *gophercloud.ProviderClient, request *providerGRPC.DeployResourceRequest, object *OpenstackObject, vars map[string]string, dependencyVars map[string]*providerGRPC.DependencyVars) (map[string]string, error) {
	logrus.Debugf("Deploying host \"%s\"", request.Resource.Key)

	// Initialize updated vars to old vars
	updatedVars := make(map[string]string)
	for k, v := range vars {
		updatedVars[k] = v
	}

	// Generate the Compute V2 client
	endpointOpts := gophercloud.EndpointOpts{
		Region: CONFIG.RegionName,
	}
	computeClient, err := openstack.NewComputeV2(authClient, endpointOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create compute client: %v", err)
	}

	var hostFlavor *flavors.Flavor = nil
	allFlavorPages, err := flavors.ListDetail(computeClient, nil).AllPages()
	if err != nil {
		return nil, fmt.Errorf("failed to get host flavor \"%s\": %v", object.Host.Flavor, err)
	}
	allFlavors, err := flavors.ExtractFlavors(allFlavorPages)
	if err != nil {
		return nil, fmt.Errorf("failed to get host flavor \"%s\": %v", object.Host.Flavor, err)
	}
	for _, fl := range allFlavors {
		if fl.Name == object.Host.Flavor || fl.ID == object.Host.Flavor {
			hostFlavor = &fl
			break
		}
	}
	if hostFlavor == nil {
		return nil, fmt.Errorf("failed to get host flavor \"%s\": flavor not found", object.Host.Flavor)
	}

	logrus.Debugf("got flavor %s (%s)", hostFlavor.Name, hostFlavor.ID)

	var hostImage *images.Image = nil
	allImagePages, err := images.ListDetail(computeClient, nil).AllPages()
	if err != nil {
		return nil, fmt.Errorf("failed to get host image \"%s\": %v", object.Host.Image, err)
	}
	allImages, err := images.ExtractImages(allImagePages)
	if err != nil {
		return nil, fmt.Errorf("failed to get host image \"%s\": %v", object.Host.Image, err)
	}
	for _, img := range allImages {
		if img.Name == object.Host.Image || img.ID == object.Host.Image {
			hostImage = &img
			break
		}
	}
	if hostImage == nil {
		return nil, fmt.Errorf("failed to get host image \"%s\": image not found", object.Host.Image)
	}

	// Check if the image requires more space than provided
	if object.Host.DiskSize < hostImage.MinDisk {
		return nil, fmt.Errorf("host disk size is too small for image (minimum %dGB required)", hostImage.MinDisk)
	}

	logrus.Debugf("got image %s (%s)", hostImage.Name, hostImage.ID)

	// Use either key or provided name as instance name
	instanceName := object.Host.Hostname
	if object.Host.Name != nil {
		instanceName = *object.Host.Name
	}
	// Prepend the first 8 bytes of deployment ID
	instanceName = request.Deployment.Id[:8] + "-" + instanceName

	// Configure the volume to clone from the image
	blockOps := []bootfromvolume.BlockDevice{
		{
			UUID:                hostImage.ID,
			BootIndex:           0,
			DeleteOnTermination: true,
			DestinationType:     bootfromvolume.DestinationVolume,
			SourceType:          bootfromvolume.SourceImage,
			VolumeSize:          object.Host.DiskSize,
		},
	}

	// Create network mappings for each network attachment
	hostNetworks := []servers.Network{}
	for k, networkAttachment := range object.Host.Networks {
		// Extract the network vars from dependencyVars
		networkVars, ok := dependencyVars[k]
		if !ok {
			return nil, fmt.Errorf("failed to get vars for network %s", k)
		}

		networkId, exists := networkVars.Vars["id"]
		if !exists {
			return nil, fmt.Errorf("ID unknown for network \"%s\"", k)
		}
		PLACEHOLDER_NET := servers.Network{
			UUID: networkId,
		}
		if !networkAttachment.DHCP && networkAttachment.IP != nil {
			PLACEHOLDER_NET.FixedIP = networkAttachment.IP.String()
		}
		hostNetworks = append(hostNetworks, PLACEHOLDER_NET)
	}

	// Configure the instance options
	hostOps := servers.CreateOpts{
		Name:      instanceName,
		ImageRef:  hostImage.ID,
		FlavorRef: hostFlavor.ID,
		UserData:  object.Host.UserData,
		Networks:  hostNetworks,
	}

	// Create the host
	createOpts := bootfromvolume.CreateOptsExt{
		CreateOptsBuilder: hostOps,
		BlockDevice:       blockOps,
	}
	deployedServer, err := bootfromvolume.Create(computeClient, createOpts).Extract()
	if err != nil {
		return nil, fmt.Errorf("failed to deploy host: %v", err)
	}

	// Save the deployed host into vars
	updatedVars["id"] = deployedServer.ID

	// Wait for server to be in ACTIVE state
	for {
		// Get the updated server from Openstack
		deployedServer, err = servers.Get(computeClient, deployedServer.ID).Extract()
		if err != nil {
			return nil, fmt.Errorf("failed to get openstack server status: %v", err)
		}
		if deployedServer.Status == "ERROR" {
			// Something happened and this failed
			return nil, fmt.Errorf("failed to deploy host: server in ERROR state")
		}
		if deployedServer.Status == "ACTIVE" {
			// Server deployed properly
			break
		}
		// Wait 5 seconds before checking again
		time.Sleep(5 * time.Second)
	}

	logrus.Debugf("Successfully deployed host %s as server %s (%s)", request.Resource.Key, deployedServer.Name, deployedServer.ID)

	return updatedVars, nil
}

func (provider *ProviderOpenstack) deployNetwork(ctx context.Context, authClient *gophercloud.ProviderClient, request *providerGRPC.DeployResourceRequest, object *OpenstackObject, vars map[string]string, dependencyVars map[string]*providerGRPC.DependencyVars) (map[string]string, error) {
	logrus.Debugf("Deploying network \"%s\"", request.Resource.Key)

	// Initialize updated vars to old vars
	updatedVars := make(map[string]string)
	for k, v := range vars {
		updatedVars[k] = v
	}

	// Generate the Network V2 client
	endpointOpts := gophercloud.EndpointOpts{
		Name:   "neutron",
		Region: CONFIG.RegionName,
	}
	networkClient, err := openstack.NewNetworkV2(authClient, endpointOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create openstack network client: %v", err)
	}

	networkName := request.Resource.Key
	if object.Network.Name != nil {
		networkName = *object.Network.Name
	}
	// Prepend the first 8 bytes of deployment ID
	networkName = request.Deployment.Id[:8] + "-" + networkName

	// Create the network
	deployedNetwork, err := networks.Create(networkClient, networks.CreateOpts{
		Name:         networkName,
		AdminStateUp: gophercloud.Enabled,
	}).Extract()
	if err != nil {
		return nil, fmt.Errorf("failed to create network: %v", err)
	}

	// Save the deployed network id into vars
	updatedVars["id"] = deployedNetwork.ID

	// Configure the subnet on the network
	var gatewayIp *string = nil
	if object.Network.Gateway != nil {
		gatewayString := object.Network.Gateway.String()
		gatewayIp = &gatewayString
	}
	dhcpPools := []subnets.AllocationPool{}
	for _, dhcp := range object.Network.DHCP {
		dhcpPools = append(dhcpPools, subnets.AllocationPool{
			Start: dhcp.Start.String(),
			End:   dhcp.End.String(),
		})
	}
	dnsServers := []string{}
	for _, resolverIP := range object.Network.Resolvers {
		dnsServers = append(dnsServers, resolverIP.String())
	}

	// Create openstack subnet on network
	deployedSubnet, err := subnets.Create(networkClient, subnets.CreateOpts{
		NetworkID:       deployedNetwork.ID,
		CIDR:            object.Network.Subnet.String(),
		Name:            networkName,
		Description:     fmt.Sprintf("%s Subnet for Network \"%s\"", object.Network.Subnet.String(), networkName),
		AllocationPools: dhcpPools,
		GatewayIP:       gatewayIp,
		IPVersion:       gophercloud.IPv4,
		EnableDHCP:      gophercloud.Enabled,
		DNSNameservers:  dnsServers,
	}).Extract()
	if err != nil {
		return nil, fmt.Errorf("failed to create subnet: %v", err)
	}

	// Save the deployed network subnet id into vars
	updatedVars["subnet_id"] = deployedSubnet.ID

	logrus.Debugf("Successfully deployed network %s as network %s (%s)", request.Resource.Key, deployedNetwork.Name, deployedNetwork.ID)

	return updatedVars, nil
}

func (provider *ProviderOpenstack) deployRouter(ctx context.Context, authClient *gophercloud.ProviderClient, request *providerGRPC.DeployResourceRequest, object *OpenstackObject, vars map[string]string, dependencyVars map[string]*providerGRPC.DependencyVars) (map[string]string, error) {
	logrus.Debugf("Deploying router \"%s\"", request.Resource.Key)

	// Initialize updated vars to old vars
	updatedVars := make(map[string]string)
	for k, v := range vars {
		updatedVars[k] = v
	}

	// Generate the Network V2 client
	endpointOpts := gophercloud.EndpointOpts{
		Name:   "neutron",
		Region: CONFIG.RegionName,
	}
	networkClient, err := openstack.NewNetworkV2(authClient, endpointOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create openstack network client: %v", err)
	}

	// Pull the external network ID from dependencyVars
	networkVars, ok := dependencyVars[object.Router.ExternalNetwork]
	if !ok {
		return nil, fmt.Errorf("failed to get vars for external network %s", object.Router.ExternalNetwork)
	}
	externalNetworkId, exists := networkVars.Vars["id"]
	if !exists {
		return nil, fmt.Errorf("ID unknown for network \"%s\"", object.Router.ExternalNetwork)
	}

	// Prepend the first 8 bytes of deployment ID
	routerName := request.Deployment.Id[:8] + "-" + request.Resource.Key

	routerConfig := routers.CreateOpts{
		Name:         routerName,
		Description:  "",
		AdminStateUp: gophercloud.Enabled,
		GatewayInfo: &routers.GatewayInfo{
			NetworkID: externalNetworkId,
		},
	}
	if object.Router.Name != nil {
		routerConfig.Name = *object.Router.Name
	}
	if object.Router.Description != nil {
		routerConfig.Description = *object.Router.Description
	}

	// Deploy the router
	deployedRouter, err := routers.Create(networkClient, routerConfig).Extract()
	if err != nil {
		return nil, fmt.Errorf("failed to create router: %v", err)
	}

	// Save the deployed router into vars
	updatedVars["id"] = deployedRouter.ID

	// Connect router to all attached networks
	for k, networkAttachment := range object.Router.Networks {
		// Extract the network vars from dependencyVars
		networkVars, ok := dependencyVars[k]
		if !ok {
			return nil, fmt.Errorf("failed to get vars for network %s", k)
		}

		// Get network and subnet ID's
		networkId, exists := networkVars.Vars["id"]
		if !exists {
			return nil, fmt.Errorf("ID unknown for network \"%s\"", k)
		}
		networkSubnetId, exists := networkVars.Vars["subnet_id"]
		if !exists {
			return nil, fmt.Errorf("ID unknown for network \"%s\" subnet", k)
		}
		// Create Openstack port for router on subnet
		osPort, err := ports.Create(networkClient, ports.CreateOpts{
			NetworkID:    networkId,
			AdminStateUp: gophercloud.Enabled,
			FixedIPs: []ports.IP{{
				SubnetID:  networkSubnetId,
				IPAddress: networkAttachment.IP.String(),
			}},
		}).Extract()
		if err != nil {
			return nil, fmt.Errorf("failed to create port for router: %v", err)
		}

		// Save the deployed router network port into vars
		updatedVars[k+"_port_id"] = osPort.ID

		// We don't need to store this ID since it will get auto-deleted on router delete
		_, err = routers.AddInterface(networkClient, deployedRouter.ID, routers.AddInterfaceOpts{
			PortID: osPort.ID,
		}).Extract()
		if err != nil {
			return nil, fmt.Errorf("failed to create router interface: %v", err)
		}
	}

	logrus.Debugf("Successfully deployed router %s as router %s (%s)", request.Resource.Key, deployedRouter.Name, deployedRouter.ID)

	return updatedVars, nil
}
