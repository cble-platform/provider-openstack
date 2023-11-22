package openstack

import (
	"context"
	"fmt"
	"sync"
	"time"

	commonGRPC "github.com/cble-platform/cble-provider-grpc/pkg/common"
	providerGRPC "github.com/cble-platform/cble-provider-grpc/pkg/provider"
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/baremetal/v1/ports"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/layer3/routers"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/subnets"
	"github.com/sirupsen/logrus"
)

func (provider ProviderOpenstack) Destroy(ctx context.Context, request *providerGRPC.DestroyRequest) (*providerGRPC.DestroyReply, error) {
	logrus.Debugf("Destroy called for deployment \"%s\"", request.DeploymentId)

	// Check if the provider has been configured
	if CONFIG == nil {
		return nil, fmt.Errorf("cannot destroy with unconfigured provider, please call Configure()")
	}

	// Generate authenticated client session
	authClient, err := provider.newAuthClient()
	if err != nil {
		return nil, fmt.Errorf("failed to authenticate: %v", err)
	}

	// Parse blueprint into struct
	blueprint, err := UnmarshalBlueprintBytesWithVars(request.Blueprint, request.TemplateVars.AsMap())
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal blueprint: %v", err)
	}

	// Validate the blueprint is valid
	err = ValidateBlueprint(blueprint)
	if err != nil {
		return nil, fmt.Errorf("blueprint is invalid: %v", err)
	}

	// Generate local routine-safe var map
	var varMap sync.Map
	for k, v := range request.DeploymentVars.AsMap() {
		varMap.Store(k, v)
	}
	// Generate local routine-safe state map
	var stateMap sync.Map
	for k, v := range request.DeploymentState.AsMap() {
		stateMap.Store(k, v)
	}

	response := providerGRPC.DestroyReply{
		Status: commonGRPC.RPCStatus_SUCCESS,
	}

	objectsWg := sync.WaitGroup{}
	for k := range blueprint.Objects {
		objectsWg.Add(1)
		go func(key string) {
			// Wait until all depends_on are done
			err := awaitRequiredBy(blueprint, &stateMap, key)
			if err != nil {
				logrus.Errorf("failed to wait on dependents: %v", err)
			} else {
				switch blueprint.Objects[key].Resource {
				case OpenstackResourceTypeHost:
					if err := provider.destroyHost(ctx, authClient, &varMap, &stateMap, blueprint, key); err != nil {
						response.Status = commonGRPC.RPCStatus_FAILURE
						response.Errors = append(response.Errors, fmt.Sprintf("failed to destroy host \"%s\": %v", key, err))
					}
				case OpenstackResourceTypeNetwork:
					if err := provider.destroyNetwork(ctx, authClient, &varMap, &stateMap, blueprint, key); err != nil {
						response.Status = commonGRPC.RPCStatus_FAILURE
						response.Errors = append(response.Errors, fmt.Sprintf("failed to destroy network \"%s\": %v", key, err))
					}
				case OpenstackResourceTypeRouter:
					if err := provider.destroyRouter(ctx, authClient, &varMap, &stateMap, blueprint, key); err != nil {
						response.Status = commonGRPC.RPCStatus_FAILURE
						response.Errors = append(response.Errors, fmt.Sprintf("failed to destroy router \"%s\": %v", key, err))
					}
				}
			}
			objectsWg.Done()
		}(k)
	}
	objectsWg.Wait()

	return &response, nil
}

func (provider *ProviderOpenstack) destroyNetwork(ctx context.Context, authClient *gophercloud.ProviderClient, varMap *sync.Map, stateMap *sync.Map, blueprint *OpenstackBlueprint, networkKey string) error {
	logrus.Debugf("Destroying network \"%s\"", networkKey)

	var err error
	// Get the network from blueprint
	_, exist := blueprint.Networks[networkKey]
	if !exist {
		stateMap.Store(networkKey, commonGRPC.DeployStateFAILED)
		return fmt.Errorf("network \"%s\" is not defined", networkKey)
	}

	// Set network as in progress for dependencies
	stateMap.Store(networkKey, commonGRPC.DeployStateINPROGRESS)

	// Generate the Network V2 client
	endpointOpts := gophercloud.EndpointOpts{
		Name:   "neutron",
		Region: CONFIG.RegionName,
	}
	networkClient, err := openstack.NewNetworkV2(authClient, endpointOpts)
	if err != nil {
		stateMap.Store(networkKey, commonGRPC.DeployStateFAILED)
		return fmt.Errorf("failed to create openstack network client: %v", err)
	}

	// Delete Openstack subnet if it exists
	osSubnetId, exists := varMap.Load(networkKey + "_subnet_id")
	if exists {
		err = subnets.Delete(networkClient, osSubnetId.(string)).ExtractErr()
		if err != nil {
			stateMap.Store(networkKey, commonGRPC.DeployStateFAILED)
			return fmt.Errorf("failed to delete subnet: %v", err)
		}
		// Wait for the subnet to be deleted
		for {
			// Attempt to get the subnet
			_, err := subnets.Get(networkClient, osSubnetId.(string)).Extract()
			if err != nil {
				// Server is deleted
				break
			}
			// Wait 5 seconds before checking again
			time.Sleep(5 * time.Second)
		}

		// Remove from vars
		varMap.Delete(networkKey + "_subnet_id")
	} else {
		stateMap.Store(networkKey, commonGRPC.DeployStateFAILED)
		return fmt.Errorf("failed to retrieve subnet id")
	}

	// Delete Openstack network if it exists
	osNetworkId, exists := varMap.Load(networkKey + "_id")
	if exists {
		err = networks.Delete(networkClient, osNetworkId.(string)).ExtractErr()
		if err != nil {
			return fmt.Errorf("failed to delete network: %v", err)
		}
		// Wait for the network to be deleted
		for {
			// Attempt to get the network
			_, err := subnets.Get(networkClient, osNetworkId.(string)).Extract()
			if err != nil {
				// Server is deleted
				break
			}
			// Wait 5 seconds before checking again
			time.Sleep(5 * time.Second)
		}

		// Remove from vars
		varMap.Delete(networkKey + "_id")
	} else {
		stateMap.Store(networkKey, commonGRPC.DeployStateFAILED)
		return fmt.Errorf("failed to retrieve network id")
	}

	logrus.Debugf("Successfully destroyed network %s", networkKey)
	return nil
}

func (provider *ProviderOpenstack) destroyRouter(ctx context.Context, authClient *gophercloud.ProviderClient, varMap *sync.Map, stateMap *sync.Map, blueprint *OpenstackBlueprint, routerKey string) error {
	var err error
	// Get the router from blueprint
	router, exist := blueprint.Routers[routerKey]
	if !exist {
		stateMap.Store(routerKey, commonGRPC.DeployStateFAILED)
		return fmt.Errorf("router \"%s\" is not defined", routerKey)
	}

	// Set router as in progress for dependencies
	stateMap.Store(routerKey, commonGRPC.DeployStateINPROGRESS)

	// Generate the Network V2 client
	endpointOpts := gophercloud.EndpointOpts{
		Name:   "neutron",
		Region: CONFIG.RegionName,
	}
	networkClient, err := openstack.NewNetworkV2(authClient, endpointOpts)
	if err != nil {
		stateMap.Store(routerKey, commonGRPC.DeployStateFAILED)
		return fmt.Errorf("failed to create openstack network client: %v", err)
	}

	osRouterId, exists := varMap.Load(routerKey + "_id")
	if exists {
		// Delete Openstack router interfaces
		for k := range router.Networks {
			osPortId, exists := varMap.Load(routerKey + "_" + k + "_port_id")
			if exists {
				_, err = routers.RemoveInterface(networkClient, osRouterId.(string), routers.RemoveInterfaceOpts{
					PortID: osPortId.(string),
				}).Extract()
				if err != nil {
					stateMap.Store(routerKey, commonGRPC.DeployStateFAILED)
					return fmt.Errorf("failed to delete router interface: %v", err)
				}
				// Wait for the router port to be deleted
				for {
					// Attempt to get the port
					_, err := ports.Get(networkClient, osPortId.(string)).Extract()
					if err != nil {
						// Port is deleted
						break
					}
					// Wait 5 seconds before checking again
					time.Sleep(5 * time.Second)
				}
				// Remove from vars
				varMap.Delete(routerKey + "_" + k + "_port_id")
				if err != nil {
					stateMap.Store(routerKey, commonGRPC.DeployStateFAILED)
					return fmt.Errorf("failed to update deployment vars: %v", err)
				}
			}
		}

		// Delete Openstack router
		err = routers.Delete(networkClient, osRouterId.(string)).ExtractErr()
		if err != nil {
			stateMap.Store(routerKey, commonGRPC.DeployStateFAILED)
			return fmt.Errorf("failed to delete router: %v", err)
		}
		// Wait for the router to be deleted
		for {
			// Attempt to get the router
			_, err := routers.Get(networkClient, osRouterId.(string)).Extract()
			if err != nil {
				// Router is deleted
				break
			}
			// Wait 5 seconds before checking again
			time.Sleep(5 * time.Second)
		}

		// Remove from vars
		varMap.Delete(routerKey + "_id")
		if err != nil {
			stateMap.Store(routerKey, commonGRPC.DeployStateFAILED)
			return fmt.Errorf("failed to update deployment vars: %v", err)
		}
	} else {
		stateMap.Store(routerKey, commonGRPC.DeployStateFAILED)
		return fmt.Errorf("failed to retrieve router id")
	}

	// Set router as destroyed for dependencies
	stateMap.Store(routerKey, commonGRPC.DeployStateDESTROYED)

	logrus.Debugf("Successfully destroyed router %s", routerKey)
	return nil
}

func (provider *ProviderOpenstack) destroyHost(ctx context.Context, authClient *gophercloud.ProviderClient, varMap *sync.Map, stateMap *sync.Map, blueprint *OpenstackBlueprint, hostKey string) error {
	var err error
	// Get the host from blueprint
	_, exist := blueprint.Hosts[hostKey]
	if !exist {
		stateMap.Store(hostKey, commonGRPC.DeployStateFAILED)
		return fmt.Errorf("host \"%s\" is not defined", hostKey)
	}

	// Set host as in progress for dependencies
	stateMap.Store(hostKey, commonGRPC.DeployStateINPROGRESS)

	// Generate the Compute V2 client
	endpointOpts := gophercloud.EndpointOpts{
		Region: CONFIG.RegionName,
	}
	computeClient, err := openstack.NewComputeV2(authClient, endpointOpts)
	if err != nil {
		stateMap.Store(hostKey, commonGRPC.DeployStateFAILED)
		return fmt.Errorf("failed to create compute v2 client: %v", err)
	}

	// Delete Openstack server if it exists
	osServerId, exists := varMap.Load(hostKey + "_id")
	if exists {
		err = servers.Delete(computeClient, osServerId.(string)).ExtractErr()
		if err != nil {
			stateMap.Store(hostKey, commonGRPC.DeployStateFAILED)
			return fmt.Errorf("failed to delete server: %v", err)
		}
		// Wait for the server to be deleted
		for {
			// Attempt to get the server
			_, err := servers.Get(computeClient, osServerId.(string)).Extract()
			if err != nil {
				// Server is deleted
				break
			}
			// Wait 5 seconds before checking again
			time.Sleep(5 * time.Second)
		}

		// Remove from vars
		varMap.Delete(hostKey + "_id")
		if err != nil {
			return fmt.Errorf("failed to update deployment vars: %v", err)
		}
	} else {
		stateMap.Store(hostKey, commonGRPC.DeployStateFAILED)
		return fmt.Errorf("failed to retrieve server id")
	}

	// Set host as destroyed for dependencies
	stateMap.Store(hostKey, commonGRPC.DeployStateDESTROYED)

	logrus.Debugf("Successfully destroyed host %s", hostKey)
	return nil
}

// Blocks execution until all objects depends_on this one are done.
func awaitRequiredBy(blueprint *OpenstackBlueprint, stateMap *sync.Map, key string) error {
	// Check on dependencies
	for {
		waitingOnDependents := false
		for _, requiredByKey := range blueprint.Objects[key].RequiredBy {
			dependentDeploymentValue, exists := stateMap.Load(requiredByKey)
			if exists {
				dependentDeploymentState := dependentDeploymentValue.(string)
				if dependentDeploymentState == commonGRPC.DeployStateFAILED {
					return fmt.Errorf("\"%s\" dependent \"%s\" failed", key, requiredByKey)
				} else if dependentDeploymentState == commonGRPC.DeployStateDESTROYED {
					// Dependent is destroyed so we're good
					continue
				} else {
					logrus.Debugf("\"%s\" is waiting on \"%s\"", key, requiredByKey)
					// early break since no need to check others if a single dependency is still inactive
					waitingOnDependents = true
					break
				}
			} else {
				waitingOnDependents = true
			}
		}
		// If all depends on objects are done
		if !waitingOnDependents {
			break
		}
		// Wait 5 secs before checking again
		time.Sleep(5 * time.Second)
	}
	return nil
}
