package openstack

import (
	"context"

	pgrpc "github.com/cble-platform/cble-provider-grpc/pkg/provider"
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

	// Convert the resource list into a key:resource map
	resourceMap := make(map[string]*pgrpc.Resource)
	for _, resource := range request.Resources {
		resourceMap[resource.Key] = resource
	}

	// Initialize empty reply
	reply := &pgrpc.ExtractResourceMetadataReply{
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

		// Check the object type
		var t *OpenstackResourceType
		if object.Resource != nil {
			t = object.Resource
		} else if object.Data != nil {
			t = object.Data
		} else {
			return extractResourceMetadataErrorReply("object needs either resource or data type"), nil
		}

		// Generate metadata based on type
		switch *t {
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
		// NETWORK
		case OpenstackResourceTypeNetwork:
			logrus.Debugf("Resource is type network")

			// Set network features
			reply.Metadata[resource.Key].Features = &pgrpc.Features{
				Power:   false,
				Console: false,
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

	return reply, nil
}
