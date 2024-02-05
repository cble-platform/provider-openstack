package openstack

import (
	"context"
	"fmt"
	"time"

	providerGRPC "github.com/cble-platform/cble-provider-grpc/pkg/provider"
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/baremetal/v1/ports"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/layer3/routers"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/subnets"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

func (provider ProviderOpenstack) DestroyResource(ctx context.Context, request *providerGRPC.DestroyResourceRequest) (*providerGRPC.DestroyResourceReply, error) {
	logrus.Debugf("----- DestroyResource called for deployment (%s) resource %s -----", request.Deployment.Id, request.Resource.Key)

	// Check if the provider has been configured
	if CONFIG == nil {
		return &providerGRPC.DestroyResourceReply{
			Success: false,
			Errors:  Errorf("cannot destroy with unconfigured provider, please call Configure()"),
		}, nil
	}

	// Generate authenticated client session
	authClient, err := provider.newAuthClient()
	if err != nil {
		return &providerGRPC.DestroyResourceReply{
			Success: false,
			Errors:  Errorf("failed to authenticate: %v", err),
		}, nil
	}

	// Unmarshal the object YAML as struct
	var object *OpenstackObject
	err = yaml.Unmarshal(request.Resource.Object, &object)
	if err != nil {
		return &providerGRPC.DestroyResourceReply{
			Success: false,
			Errors:  Errorf("failed to unmarshal resource object: %v", err),
		}, nil
	}

	// Check this is a resource (not data)
	if object.Resource == nil {
		return &providerGRPC.DestroyResourceReply{
			Success: false,
			Errors:  Errorf("cannot destroy data object"),
		}, nil
	}

	var updatedVars map[string]string

	// Destroy the resource based on the type of resource
	switch *object.Resource {
	// HOST
	case OpenstackResourceTypeHost:
		// Destroy host
		if updatedVars, err = provider.destroyHost(ctx, authClient, request, object, request.Vars); err != nil {
			return &providerGRPC.DestroyResourceReply{
				Success: false,
				Errors:  Errorf("failed to destroy host: %v", err),
			}, nil
		}
	// NETWORK
	case OpenstackResourceTypeNetwork:
		// Destroy network
		if updatedVars, err = provider.destroyNetwork(ctx, authClient, request, object, request.Vars); err != nil {
			return &providerGRPC.DestroyResourceReply{
				Success: false,
				Errors:  Errorf("failed to destroy network: %v", err),
			}, nil
		}
	// ROUTER
	case OpenstackResourceTypeRouter:
		// Destroy router
		if updatedVars, err = provider.destroyRouter(ctx, authClient, request, object, request.Vars); err != nil {
			return &providerGRPC.DestroyResourceReply{
				Success: false,
				Errors:  Errorf("failed to destroy router: %v", err),
			}, nil
		}
	}

	// Return the updated vars
	return &providerGRPC.DestroyResourceReply{
		Success:     true,
		Errors:      nil,
		UpdatedVars: updatedVars,
	}, nil
}

func (provider *ProviderOpenstack) destroyHost(ctx context.Context, authClient *gophercloud.ProviderClient, request *providerGRPC.DestroyResourceRequest, object *OpenstackObject, vars map[string]string) (map[string]string, error) {
	logrus.Debugf("Destroying host \"%s\"", request.Resource.Key)

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

	// Get the Openstack server ID from vars
	osServerId, ok := vars["id"]
	if ok {
		// Delete the server if exists
		err = servers.Delete(computeClient, osServerId).ExtractErr()
		if err != nil {
			return nil, fmt.Errorf("failed to delete server: %v", err)
		}

		// Wait for the server to be fully deleted
		for {
			_, err = servers.Get(computeClient, osServerId).Extract()
			if err != nil {
				// Server is deleted
				break
			}
			// Check every second
			time.Sleep(time.Second)
		}

		// Remove server ID from the vars
		delete(updatedVars, "id")
	}

	logrus.Debugf("Successfully destroyed host %s", request.Resource.Key)

	return updatedVars, nil
}

func (provider *ProviderOpenstack) destroyNetwork(ctx context.Context, authClient *gophercloud.ProviderClient, request *providerGRPC.DestroyResourceRequest, object *OpenstackObject, vars map[string]string) (map[string]string, error) {
	logrus.Debugf("Destroying network \"%s\"", request.Resource.Key)

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

	// Get the Openstack subnet ID from vars
	osSubnetId, ok := vars["subnet_id"]
	if ok {
		// Delete the subnet if exists

		// Delete the Openstack subnet
		err = subnets.Delete(networkClient, osSubnetId).ExtractErr()
		if err != nil {
			return nil, fmt.Errorf("failed to delete subnet: %v", err)
		}

		// Wait for the subnet to be fully deleted
		for {
			_, err = subnets.Get(networkClient, osSubnetId).Extract()
			if err != nil {
				// Subnet is deleted
				break
			}
			// Check every second
			time.Sleep(time.Second)
		}

		// Remove subnet ID from the vars
		delete(updatedVars, "subnet_id")
	}

	// Get the Openstack network ID from vars
	osNetworkId, ok := vars["id"]
	if ok {
		// Delete the network if exists
		err = networks.Delete(networkClient, osNetworkId).ExtractErr()
		if err != nil {
			return nil, fmt.Errorf("failed to delete network: %v", err)
		}

		// Wait for the network to be fully deleted
		for {
			_, err = networks.Get(networkClient, osNetworkId).Extract()
			if err != nil {
				// Network is deleted
				break
			}
			// Check every second
			time.Sleep(time.Second)
		}

		// Remove network ID from the vars
		delete(updatedVars, "id")
	}

	logrus.Debugf("Successfully destroyed network %s", request.Resource.Key)

	return updatedVars, nil
}

func (provider *ProviderOpenstack) destroyRouter(ctx context.Context, authClient *gophercloud.ProviderClient, request *providerGRPC.DestroyResourceRequest, object *OpenstackObject, vars map[string]string) (map[string]string, error) {
	logrus.Debugf("Destroying router \"%s\"", request.Resource.Key)

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

	// Get the Openstack router ID from vars
	osRouterId, ok := vars["id"]
	if ok {
		// Delete the router if exists

		// Remove router from every network (delete all ports)
		for k := range object.Router.Networks {
			// Get the Openstack router port ID (for this network) from vars
			osPortId, ok := vars[k+"_port_id"]
			if ok {
				// Delete the router port if exists
				_, err = routers.RemoveInterface(networkClient, osRouterId, routers.RemoveInterfaceOpts{
					PortID: osPortId,
				}).Extract()
				if err != nil {
					return nil, fmt.Errorf("failed to delete router port: %v", err)
				}

				// Wait for the router port to be fully deleted
				for {
					_, err = ports.Get(networkClient, osPortId).Extract()
					if err != nil {
						// Router port is deleted
						break
					}
					// Check every second
					time.Sleep(time.Second)
				}

				// Remove router port ID from the vars
				delete(updatedVars, k+"_port_id")
			}
		}

		// Delete the Openstack router
		err = routers.Delete(networkClient, osRouterId).ExtractErr()
		if err != nil {
			return nil, fmt.Errorf("failed to delete network: %v", err)
		}

		// Wait for the router to be fully deleted
		for {
			_, err = routers.Get(networkClient, osRouterId).Extract()
			if err != nil {
				// Router is deleted
				break
			}
			// Check every second
			time.Sleep(time.Second)
		}

		// Remove router ID from the vars
		delete(updatedVars, "id")
	}

	logrus.Debugf("Successfully destroyed router %s", request.Resource.Key)

	return updatedVars, nil
}
