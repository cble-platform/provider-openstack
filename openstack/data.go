package openstack

import (
	"context"
	"fmt"
	"regexp"

	pgrpc "github.com/cble-platform/cble-provider-grpc/pkg/provider"
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/layer3/routers"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/subnets"
	"github.com/gophercloud/gophercloud/pagination"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

func (provider ProviderOpenstack) RetrieveData(ctx context.Context, request *pgrpc.RetrieveDataRequest) (*pgrpc.RetrieveDataReply, error) {
	logrus.Debugf("----- RetrieveData called for deployment (%s) resource %s -----", request.Deployment.Id, request.Resource.Key)

	// Check if the provider has been configured
	if CONFIG == nil {
		return &pgrpc.RetrieveDataReply{
			Success: false,
			Error:   Errorf("cannot deploy with unconfigured provider, please call Configure()"),
		}, nil
	}

	// Generate authenticated client session
	authClient, err := provider.newAuthClient()
	if err != nil {
		return &pgrpc.RetrieveDataReply{
			Success: false,
			Error:   Errorf("failed to authenticate: %v", err),
		}, nil
	}

	// Unmarshal the object YAML as struct
	var object *OpenstackObject
	err = yaml.Unmarshal(request.Resource.Object, &object)
	if err != nil {
		return &pgrpc.RetrieveDataReply{
			Success: false,
			Error:   Errorf("failed to unmarshal resource object: %v", err),
		}, nil
	}

	// Check this is a data (not resource)
	if object.Data == nil {
		return &pgrpc.RetrieveDataReply{
			Success: false,
			Error:   Errorf("cannot retrieve data for resource object"),
		}, nil
	}

	var updatedVars map[string]string

	// Deploy the resource based on the type of data
	switch *object.Data {
	// HOST
	case OpenstackResourceTypeHost:
		// Deploy host
		if updatedVars, err = provider.retrieveHostData(ctx, authClient, request, object, request.Vars, request.DependencyVars); err != nil {
			return &pgrpc.RetrieveDataReply{
				Success: false,
				Error:   Errorf("failed to retrieve host data: %v", err),
			}, nil
		}
	// NETWORK
	case OpenstackResourceTypeNetwork:
		// Deploy network
		if updatedVars, err = provider.retrieveNetworkData(ctx, authClient, request, object, request.Vars, request.DependencyVars); err != nil {
			return &pgrpc.RetrieveDataReply{
				Success: false,
				Error:   Errorf("failed to retrieve network data: %v", err),
			}, nil
		}
	// ROUTER
	case OpenstackResourceTypeRouter:
		// Deploy router
		if updatedVars, err = provider.retrieveRouterData(ctx, authClient, request, object, request.Vars, request.DependencyVars); err != nil {
			return &pgrpc.RetrieveDataReply{
				Success: false,
				Error:   Errorf("failed to retrieve router data: %v", err),
			}, nil
		}
	}

	// Return the updated vars
	return &pgrpc.RetrieveDataReply{
		Success:     true,
		Error:       nil,
		UpdatedVars: updatedVars,
	}, nil
}

func (provider *ProviderOpenstack) retrieveHostData(ctx context.Context, authClient *gophercloud.ProviderClient, request *pgrpc.RetrieveDataRequest, object *OpenstackObject, vars map[string]string, dependencyVars map[string]*pgrpc.DependencyVars) (map[string]string, error) {
	logrus.Debugf("Retrieving host data \"%s\"", request.Resource.Key)

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

	var openstackServer *servers.Server

	// If ID is present, just get server by id
	if object.Host.ID != nil {
		openstackServer, err = servers.Get(computeClient, *object.Host.ID).Extract()
		if err != nil {
			return nil, fmt.Errorf("failed to get server by ID: %v", err)
		}
	} else {
		listOpts := servers.ListOpts{}
		// Filter on name
		if object.Host.Name != nil {
			listOpts.Name = *object.Host.Name
		} else if object.Host.Hostname != "" {
			listOpts.Name = object.Host.Hostname
		}
		// Filter on IP regex
		if len(object.Host.Networks) > 0 {
			ipRegex := "(" // Add open group tag
			i := 0
			for _, network := range object.Host.Networks {
				if network.IP != nil {
					ipRegex = ipRegex + fmt.Sprintf("(%s)", regexp.QuoteMeta(network.IP.String()))
					// Add an OR regex if not last IP
					if i < len(object.Host.Networks)-1 {
						ipRegex = ipRegex + "|"
					}
				}
				i++
			}
			ipRegex = ipRegex + ")" // Add closing group tag
			listOpts.IP = ipRegex
		}
		err = servers.List(computeClient, listOpts).EachPage(func(p pagination.Page) (bool, error) {
			s, err := servers.ExtractServers(p)
			if err != nil {
				return false, fmt.Errorf("failed to extract server pages")
			}

			// Return the first result
			if len(s) > 0 {
				openstackServer = &s[0]
				return false, nil
			} else {
				return false, fmt.Errorf("failed to set server from page")
			}
		})
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve server: %s", err)
		}
	}

	updatedVars["id"] = openstackServer.ID

	logrus.Debugf("Successfully retrieved host %s as server %s (%s)", request.Resource.Key, openstackServer.Name, openstackServer.ID)

	return updatedVars, nil
}

func (provider *ProviderOpenstack) retrieveNetworkData(ctx context.Context, authClient *gophercloud.ProviderClient, request *pgrpc.RetrieveDataRequest, object *OpenstackObject, vars map[string]string, dependencyVars map[string]*pgrpc.DependencyVars) (map[string]string, error) {
	logrus.Debugf("Retrieving network data \"%s\"", request.Resource.Key)

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

	var openstackNetwork *networks.Network

	// If ID is present, just get network by id
	if object.Network.ID != nil {
		openstackNetwork, err = networks.Get(networkClient, *object.Network.ID).Extract()
		if err != nil {
			return nil, fmt.Errorf("failed to get network by ID: %v", err)
		}
	} else {
		listOpts := networks.ListOpts{}
		// Filter on name
		if object.Network.Name != nil {
			listOpts.Name = *object.Network.Name
		}
		err = networks.List(networkClient, listOpts).EachPage(func(p pagination.Page) (bool, error) {
			n, err := networks.ExtractNetworks(p)
			if err != nil {
				return false, fmt.Errorf("failed to extract network pages")
			}

			// Return the first result
			if len(n) > 0 {
				openstackNetwork = &n[0]
				return false, nil
			} else {
				return false, fmt.Errorf("failed to set network from page")
			}
		})
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve network: %s", err)
		}
	}

	updatedVars["id"] = openstackNetwork.ID

	var openstackSubnet *subnets.Subnet

	// Get the subnets of the network
	err = subnets.List(networkClient, subnets.ListOpts{
		NetworkID: openstackNetwork.ID,
	}).EachPage(func(p pagination.Page) (bool, error) {
		s, err := subnets.ExtractSubnets(p)
		if err != nil {
			return false, fmt.Errorf("failed to extract subnet pages")
		}

		// Return the first result
		if len(s) > 0 {
			openstackSubnet = &s[0]
			return false, nil
		} else {
			return false, fmt.Errorf("failed to set subnet from page")
		}
	})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve subnet: %s", err)
	}

	updatedVars["subnet_id"] = openstackSubnet.ID

	logrus.Debugf("Successfully retrieved network %s as network %s (%s)", request.Resource.Key, openstackNetwork.Name, openstackNetwork.ID)

	return updatedVars, nil
}

func (provider *ProviderOpenstack) retrieveRouterData(ctx context.Context, authClient *gophercloud.ProviderClient, request *pgrpc.RetrieveDataRequest, object *OpenstackObject, vars map[string]string, dependencyVars map[string]*pgrpc.DependencyVars) (map[string]string, error) {
	logrus.Debugf("Retrieving router data \"%s\"", request.Resource.Key)

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

	var openstackRouter *routers.Router

	// If ID is present, just get router by id
	if object.Router.ID != nil {
		openstackRouter, err = routers.Get(networkClient, *object.Router.ID).Extract()
		if err != nil {
			return nil, fmt.Errorf("failed to get router by ID: %v", err)
		}
	} else {
		listOpts := routers.ListOpts{}
		// Filter on name
		if object.Router.Name != nil {
			listOpts.Name = *object.Router.Name
		}
		err = routers.List(networkClient, listOpts).EachPage(func(p pagination.Page) (bool, error) {
			n, err := routers.ExtractRouters(p)
			if err != nil {
				return false, fmt.Errorf("failed to extract router pages")
			}

			// Return the first result
			if len(n) > 0 {
				openstackRouter = &n[0]
				return false, nil
			} else {
				return false, fmt.Errorf("failed to set router from page")
			}
		})
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve router: %s", err)
		}
	}

	updatedVars["id"] = openstackRouter.ID

	logrus.Debugf("Successfully retrieved router %s as router %s (%s)", request.Resource.Key, openstackRouter.Name, openstackRouter.ID)

	return updatedVars, nil
}
