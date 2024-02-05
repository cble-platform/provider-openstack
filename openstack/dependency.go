package openstack

import (
	"context"
	"fmt"

	providerGRPC "github.com/cble-platform/cble-provider-grpc/pkg/provider"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

func (provider ProviderOpenstack) GenerateDependencies(ctx context.Context, request *providerGRPC.GenerateDependenciesRequest) (*providerGRPC.GenerateDependenciesReply, error) {
	logrus.Debugf("----- GenerateDependencies called with %d resources -----", len(request.Resources))

	// Convert the resource list into a key:resource map
	resourceMap := make(map[string]*providerGRPC.Resource)
	for _, resource := range request.Resources {
		resourceMap[resource.Key] = resource
	}

	// Generate dependencies (resource:dependency)
	dependencyList := make([]string, 0)
	for _, resource := range request.Resources {
		logrus.Debugf("Generating dependencies for resource %s", resource.Key)

		// Unmarshal the object YAML as struct
		var object OpenstackObject
		err := yaml.Unmarshal(resource.Object, &object)
		if err != nil {
			return &providerGRPC.GenerateDependenciesReply{
				Success:      false,
				Errors:       Errorf("failed to marshal object for resource %s: %v", resource.Key, err),
				Dependencies: nil,
			}, nil
		}

		// Check the object type
		var t *OpenstackResourceType
		if object.Resource != nil {
			t = object.Resource
		} else if object.Data != nil {
			t = object.Data
		} else {
			return &providerGRPC.GenerateDependenciesReply{
				Success:      false,
				Errors:       Errorf("object needs either resource or data type"),
				Dependencies: nil,
			}, nil
		}

		// Generate dependencies based on type
		switch *t {
		case OpenstackResourceTypeHost:
			logrus.Debugf("Resource is type host")
			// Add all networks host is on as dependencies
			for nk := range object.Host.Networks {
				// Check the network exists in resources
				if _, ok := resourceMap[nk]; !ok {
					return &providerGRPC.GenerateDependenciesReply{
						Success: false,
						Errors:  Errorf("host %s depends on network %s which isn't defined", resource.Key, nk),
					}, nil
				}
				logrus.Debugf("\tAdding host dependency on network %s", nk)
				dependencyList = append(dependencyList, fmt.Sprintf("%s:%s", resource.Key, nk))
			}
		case OpenstackResourceTypeRouter:
			logrus.Debugf("Resource is type router")
			// Add external network as dependency
			if _, ok := resourceMap[object.Router.ExternalNetwork]; !ok {
				return &providerGRPC.GenerateDependenciesReply{
					Success: false,
					Errors:  Errorf("router %s depends on external network %s which isn't defined", resource.Key, object.Router.ExternalNetwork),
				}, nil
			}
			logrus.Debugf("\tAdding router dependency on network %s", object.Router.ExternalNetwork)
			dependencyList = append(dependencyList, fmt.Sprintf("%s:%s", resource.Key, object.Router.ExternalNetwork))
			// Add all networks router is connected to as dependencies
			for nk := range object.Router.Networks {
				// Check the network exists in resources
				if _, ok := resourceMap[nk]; !ok {
					return &providerGRPC.GenerateDependenciesReply{
						Success: false,
						Errors:  Errorf("router %s depends on network %s which isn't defined", resource.Key, nk),
					}, nil
				}
				logrus.Debugf("\tAdding router dependency on network %s", nk)
				dependencyList = append(dependencyList, fmt.Sprintf("%s:%s", resource.Key, nk))
			}
		}

		// Add dependencies based on depends_on
		logrus.Debugf("Adding resource depends_on dependencies")
		for _, dependsOnKey := range object.DependsOn {
			// Check the network exists in resources
			if _, ok := resourceMap[dependsOnKey]; !ok {
				return &providerGRPC.GenerateDependenciesReply{
					Success: false,
					Errors:  Errorf("resource %s depends on resource %s which doesn't exist", resource.Key, dependsOnKey),
				}, nil
			}
			logrus.Debugf("\tAdding dependency %s", dependsOnKey)
			dependencyList = append(dependencyList, fmt.Sprintf("%s:%s", resource.Key, dependsOnKey))
		}
	}

	return &providerGRPC.GenerateDependenciesReply{
		Success:      true,
		Errors:       nil,
		Dependencies: dependencyList,
	}, nil
}
