package openstack

// func (provider ProviderOpenstack) GetResourceList(ctx context.Context, request *pgrpc.GetResourceListRequest) (*pgrpc.GetResourceListReply, error) {
// 	logrus.Debugf("GetResourceList called for deployment \"%s\"", request.DeploymentId)

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

// 	response := &pgrpc.GetResourceListReply{
// 		Status:       common.RPCStatus_SUCCESS,
// 		DeploymentId: request.DeploymentId,
// 		Resources:    []*pgrpc.Resource{},
// 	}

// 	for key := range blueprint.Objects {
// 		switch blueprint.Objects[key].Resource {
// 		// HOST
// 		case OpenstackResourceTypeHost:
// 			name := blueprint.Hosts[key].Hostname
// 			if blueprint.Hosts[key].Name != nil {
// 				name = *blueprint.Hosts[key].Name
// 			}
// 			response.Resources = append(response.Resources, &pgrpc.Resource{
// 				Key:          key,
// 				DeploymentId: request.DeploymentId,
// 				Name:         name,
// 				Type:         pgrpc.ResourceType_HOST,
// 			})
// 		// NETWORK
// 		case OpenstackResourceTypeNetwork:
// 			name := key
// 			if blueprint.Networks[key].Name != nil {
// 				name = *blueprint.Networks[key].Name
// 			}
// 			response.Resources = append(response.Resources, &pgrpc.Resource{
// 				Key:          key,
// 				DeploymentId: request.DeploymentId,
// 				Name:         name,
// 				Type:         pgrpc.ResourceType_NETWORK,
// 			})
// 		// ROUTER
// 		case OpenstackResourceTypeRouter:
// 			name := key
// 			if blueprint.Routers[key].Name != nil {
// 				name = *blueprint.Routers[key].Name
// 			}
// 			response.Resources = append(response.Resources, &pgrpc.Resource{
// 				Key:          key,
// 				DeploymentId: request.DeploymentId,
// 				Name:         name,
// 				Type:         pgrpc.ResourceType_ROUTER,
// 			})
// 		default:
// 			response.Resources = append(response.Resources, &pgrpc.Resource{
// 				Key:          key,
// 				DeploymentId: request.DeploymentId,
// 				Name:         key,
// 				Type:         pgrpc.ResourceType_UNKNOWN,
// 			})
// 		}
// 	}

// 	return response, nil
// }
