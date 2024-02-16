package openstack

import (
	"context"
	"fmt"

	providerGRPC "github.com/cble-platform/cble-provider-grpc/pkg/provider"
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/startstop"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

func (provider ProviderOpenstack) ResourcePower(ctx context.Context, request *providerGRPC.ResourcePowerRequest) (*providerGRPC.ResourcePowerReply, error) {
	logrus.Debugf("----- ResourcePower called for resource \"%s\" -----", request.Resource.Id)

	// Check if the provider has been configured
	if CONFIG == nil {
		return &providerGRPC.ResourcePowerReply{
			Success: false,
			Errors:  Errorf("cannot destroy with unconfigured provider, please call Configure()"),
		}, nil
	}

	// Generate authenticated client session
	authClient, err := provider.newAuthClient()
	if err != nil {
		return &providerGRPC.ResourcePowerReply{
			Success: false,
			Errors:  Errorf("failed to authenticate: %v", err),
		}, nil
	}

	// Unmarshal the object YAML as struct
	var object *OpenstackObject
	err = yaml.Unmarshal(request.Resource.Object, &object)
	if err != nil {
		return &providerGRPC.ResourcePowerReply{
			Success: false,
			Errors:  Errorf("failed to unmarshal resource object: %v", err),
		}, nil
	}

	// Check this is a resource (not data)
	if object.Resource == nil {
		return &providerGRPC.ResourcePowerReply{
			Success: false,
			Errors:  Errorf("cannot destroy data object"),
		}, nil
	}

	// Check the resource type (only allow power modifications to servers)
	if *object.Resource != OpenstackResourceTypeHost {
		return &providerGRPC.ResourcePowerReply{
			Success: false,
			Errors:  Errorf("cannot modify power state for this resource"),
		}, nil
	}

	switch request.State {
	case providerGRPC.PowerState_ON:
		err = provider.powerOnResource(ctx, authClient, request, object)
	case providerGRPC.PowerState_OFF:
		err = provider.powerOffResource(ctx, authClient, request, object)
	case providerGRPC.PowerState_RESET:
		err = provider.resetResource(ctx, authClient, request, object)
	default:
		return &providerGRPC.ResourcePowerReply{
			Success: false,
			Errors:  Errorf("power state \"%s\" is unknown", request.State),
		}, nil
	}

	if err != nil {
		return &providerGRPC.ResourcePowerReply{
			Success: false,
			Errors:  Errorf(err.Error()),
		}, nil
	}

	return &providerGRPC.ResourcePowerReply{
		Success: true,
		Errors:  nil,
	}, nil
}

func (provider ProviderOpenstack) powerOnResource(ctx context.Context, authClient *gophercloud.ProviderClient, request *providerGRPC.ResourcePowerRequest, object *OpenstackObject) error {
	logrus.Debugf("Powering on host \"%s\"", request.Resource.Id)

	// Generate the Compute V2 client
	endpointOpts := gophercloud.EndpointOpts{
		Region: CONFIG.RegionName,
	}
	computeClient, err := openstack.NewComputeV2(authClient, endpointOpts)
	if err != nil {
		return fmt.Errorf("failed to create compute client: %v", err)
	}

	// Get the Openstack server ID from vars
	osServerId, ok := request.Vars["id"]
	if ok {
		// Start the vm
		err = startstop.Start(computeClient, osServerId).ExtractErr()
		if err != nil {
			return fmt.Errorf("failed to start server: %v", err)
		}
	} else {
		return fmt.Errorf("no ID found for resource")
	}
	return nil
}

func (provider ProviderOpenstack) powerOffResource(ctx context.Context, authClient *gophercloud.ProviderClient, request *providerGRPC.ResourcePowerRequest, object *OpenstackObject) error {
	logrus.Debugf("Powering off host \"%s\"", request.Resource.Id)

	// Generate the Compute V2 client
	endpointOpts := gophercloud.EndpointOpts{
		Region: CONFIG.RegionName,
	}
	computeClient, err := openstack.NewComputeV2(authClient, endpointOpts)
	if err != nil {
		return fmt.Errorf("failed to create compute client: %v", err)
	}

	// Get the Openstack server ID from vars
	osServerId, ok := request.Vars["id"]
	if ok {
		// Start the vm
		err = startstop.Stop(computeClient, osServerId).ExtractErr()
		if err != nil {
			return fmt.Errorf("failed to stop server: %v", err)
		}
	} else {
		return fmt.Errorf("no ID found for resource")
	}
	return nil
}

func (provider ProviderOpenstack) resetResource(ctx context.Context, authClient *gophercloud.ProviderClient, request *providerGRPC.ResourcePowerRequest, object *OpenstackObject) error {
	logrus.Debugf("Resetting host \"%s\"", request.Resource.Id)

	// Generate the Compute V2 client
	endpointOpts := gophercloud.EndpointOpts{
		Region: CONFIG.RegionName,
	}
	computeClient, err := openstack.NewComputeV2(authClient, endpointOpts)
	if err != nil {
		return fmt.Errorf("failed to create compute client: %v", err)
	}

	// Get the Openstack server ID from vars
	osServerId, ok := request.Vars["id"]
	if ok {
		// Reboot the server
		err = servers.Reboot(computeClient, osServerId, servers.RebootOpts{
			Type: servers.HardReboot,
		}).ExtractErr()
		if err != nil {
			return fmt.Errorf("failed to reset server: %v", err)
		}
	} else {
		return fmt.Errorf("no ID found for resource")
	}
	return nil
}
