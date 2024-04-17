package openstack

import (
	"context"

	pgrpc "github.com/cble-platform/cble-provider-grpc/pkg/provider"
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/flavors"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

func extractResourceMetadataErrorReply(format string, a ...any) *pgrpc.ExtractResourceMetadataReply {
	return &pgrpc.ExtractResourceMetadataReply{
		Success: false,
		Error:   Errorf(format, a...),
	}
}

func (provider ProviderOpenstack) ExtractResourceMetadata(ctx context.Context, request *pgrpc.ExtractResourceMetadataRequest) (*pgrpc.ExtractResourceMetadataReply, error) {
	logrus.Debugf("----- ExtractResourceMetadata called with %d resources -----", len(request.Resources))

	// Check if the provider has been configured
	if CONFIG == nil {
		return extractResourceMetadataErrorReply("cannot deploy with unconfigured provider, please call Configure()"), nil
	}

	// Generate authenticated client session
	authClient, err := provider.newAuthClient()
	if err != nil {
		return extractResourceMetadataErrorReply("failed to authenticate: %v", err), nil
	}

	// Generate the Compute V2 client
	endpointOpts := gophercloud.EndpointOpts{
		Region: CONFIG.RegionName,
	}
	computeClient, err := openstack.NewComputeV2(authClient, endpointOpts)
	if err != nil {
		return extractResourceMetadataErrorReply("failed to create compute client: %v", err), nil
	}

	// Convert the resource list into a key:resource map
	resourceMap := make(map[string]*pgrpc.Resource)
	for _, resource := range request.Resources {
		resourceMap[resource.Key] = resource
	}

	// Initialize empty reply
	reply := &pgrpc.ExtractResourceMetadataReply{
		Success:  true,
		Metadata: map[string]*pgrpc.Metadata{},
	}

	// Generate metadata for each resource
	for _, resource := range request.Resources {
		logrus.Debugf("Extracting metadata for resource %s", resource.Key)

		// Initialize metadata for the object
		reply.Metadata[resource.Key] = &pgrpc.Metadata{
			DependsOnKeys: make([]string, 0),
		}

		// Unmarshal the object YAML as struct
		var object OpenstackObject
		err := yaml.Unmarshal(resource.Object, &object)
		if err != nil {
			return extractResourceMetadataErrorReply("failed to marshal object for resource %s: %v", resource.Key, err), nil
		}

		// Only generate metadata for resource (not needed for data)
		if object.Resource != nil {
			// Generate metadata based on type
			switch *object.Resource {
			// HOST
			case OpenstackResourceTypeHost:
				logrus.Debugf("Resource is type host")

				// Add all networks host is on as dependencies
				for nk := range object.Host.Networks {
					// Check the network exists in resources
					if _, ok := resourceMap[nk]; !ok {
						return extractResourceMetadataErrorReply("host %s depends on network %s which isn't defined", resource.Key, nk), nil
					}
					logrus.Debugf("\tAdding host dependency on network %s", nk)
					reply.Metadata[resource.Key].DependsOnKeys = append(reply.Metadata[resource.Key].DependsOnKeys, nk)
				}

				// Set host features
				reply.Metadata[resource.Key].Features = &pgrpc.Features{
					Power:   true,
					Console: true,
				}

				// Get the server flavor to determine quota requirements
				var hostFlavor *flavors.Flavor = nil
				allFlavorPages, err := flavors.ListDetail(computeClient, nil).AllPages()
				if err != nil {
					return extractResourceMetadataErrorReply("failed to get host %s flavor \"%s\": %v", resource.Key, object.Host.Flavor, err), nil
				}
				allFlavors, err := flavors.ExtractFlavors(allFlavorPages)
				if err != nil {
					return extractResourceMetadataErrorReply("failed to get host %s flavor \"%s\": %v", resource.Key, object.Host.Flavor, err), nil
				}
				for _, fl := range allFlavors {
					if fl.Name == object.Host.Flavor || fl.ID == object.Host.Flavor {
						hostFlavor = &fl
						break
					}
				}
				if hostFlavor == nil {
					return extractResourceMetadataErrorReply("failed to get host %s flavor \"%s\": flavor not found", resource.Key, object.Host.Flavor), nil
				}

				// Set the quota requirements
				reply.Metadata[resource.Key].QuotaRequirements = &pgrpc.QuotaRequirements{
					Cpu:  uint64(hostFlavor.VCPUs),
					Ram:  uint64(hostFlavor.RAM),         // Already in MiB
					Disk: uint64(hostFlavor.Disk) * 1024, // Convert GiB to MiB
				}
			// NETWORK
			case OpenstackResourceTypeNetwork:
				logrus.Debugf("Resource is type network")

				// Set network features
				reply.Metadata[resource.Key].Features = &pgrpc.Features{
					Power:   false,
					Console: false,
				}

				// Set the quota requirements
				reply.Metadata[resource.Key].QuotaRequirements = &pgrpc.QuotaRequirements{
					Network: 1,
				}
			// ROUTER
			case OpenstackResourceTypeRouter:
				logrus.Debugf("Resource is type router")

				// Add external network as dependency
				if _, ok := resourceMap[object.Router.ExternalNetwork]; !ok {
					return extractResourceMetadataErrorReply("router %s depends on external network %s which isn't defined", resource.Key, object.Router.ExternalNetwork), nil
				}
				logrus.Debugf("\tAdding router dependency on network %s", object.Router.ExternalNetwork)
				reply.Metadata[resource.Key].DependsOnKeys = append(reply.Metadata[resource.Key].DependsOnKeys, object.Router.ExternalNetwork)

				// Add all networks router is connected to as dependencies
				for nk := range object.Router.Networks {
					// Check the network exists in resources
					if _, ok := resourceMap[nk]; !ok {
						return extractResourceMetadataErrorReply("router %s depends on network %s which isn't defined", resource.Key, nk), nil
					}
					logrus.Debugf("\tAdding router dependency on network %s", nk)
					reply.Metadata[resource.Key].DependsOnKeys = append(reply.Metadata[resource.Key].DependsOnKeys, nk)
				}

				// Set router features
				reply.Metadata[resource.Key].Features = &pgrpc.Features{
					Power:   false,
					Console: false,
				}

				// Set the quota requirements
				reply.Metadata[resource.Key].QuotaRequirements = &pgrpc.QuotaRequirements{
					Router: 1,
				}
			}

			// Add dependencies based on depends_on
			logrus.Debugf("Adding resource depends_on dependencies")
			for _, dependsOnKey := range object.DependsOn {
				// Check the network exists in resources
				if _, ok := resourceMap[dependsOnKey]; !ok {
					return extractResourceMetadataErrorReply("resource %s depends on resource %s which doesn't exist", resource.Key, dependsOnKey), nil
				}
				logrus.Debugf("\tAdding dependency %s", dependsOnKey)
				reply.Metadata[resource.Key].DependsOnKeys = append(reply.Metadata[resource.Key].DependsOnKeys, dependsOnKey)
			}
		}
	}

	return reply, nil
}
