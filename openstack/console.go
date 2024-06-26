package openstack

// func GetConsoleErrorf(request *pgrpc.GetConsoleRequest, format string, a ...any) *pgrpc.GetConsoleReply {
// 	return &pgrpc.GetConsoleReply{
// 		DeploymentId: request.DeploymentId,
// 		Status:       commonGRPC.RPCStatus_FAILURE,
// 		Console:      "",
// 		Error: []string{
// 			fmt.Sprintf(format, a...),
// 		},
// 	}
// }

// func (provider ProviderOpenstack) GetConsole(ctx context.Context, request *pgrpc.GetConsoleRequest) (*pgrpc.GetConsoleReply, error) {
// 	logrus.Debugf("GetConsole called for deployment \"%s\" host \"%s\"", request.DeploymentId, request.HostKey)

// 	// Check if the provider has been configured
// 	if CONFIG == nil {
// 		return nil, fmt.Errorf("cannot deploy with unconfigured provider, please call Configure()")
// 	}

// 	// Generate authenticated client session
// 	authClient, err := provider.newAuthClient()
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to authenticate: %v", err)
// 	}

// 	// Convert protobuf maps to go maps
// 	deploymentVars := request.DeploymentVars.AsMap()
// 	deploymentState := request.DeploymentState.AsMap()

// 	// Check that the host state is good
// 	if deploymentState[request.HostKey] != commonGRPC.DeployStateSUCCEEDED {
// 		return GetConsoleErrorf(request, "host %s is not in SUCCEEDED state", request.HostKey), nil
// 	}

// 	// Check for VM ID in vars
// 	instanceId, ok := deploymentVars[request.HostKey+"_id"]
// 	if !ok {
// 		return GetConsoleErrorf(request, "host %s does not have instance id", request.HostKey), nil
// 	}
// 	// Ensure instanceId is string
// 	instanceIdStr, ok := instanceId.(string)
// 	if !ok {
// 		return GetConsoleErrorf(request, "host %s has non-string id", request.HostKey), nil
// 	}

// 	// Generate the Compute V2 client
// 	endpointOpts := gophercloud.EndpointOpts{
// 		Region: CONFIG.RegionName,
// 	}
// 	computeClient, err := openstack.NewComputeV2(authClient, endpointOpts)
// 	if err != nil {
// 		return GetConsoleErrorf(request, "failed to create compute v2 client: %v", err), nil
// 	}
// 	// Set the microversion of the compute api (min for remote consoles is 2.6)
// 	computeClient.Microversion = "2.6"

// 	// Create the remote console and return the URL
// 	remoteConsole, err := remoteconsoles.Create(computeClient, instanceIdStr, remoteconsoles.CreateOpts{
// 		Protocol: CONFIG.PreferredConsoleProtocol,
// 		Type:     CONFIG.PreferredConsoleType,
// 	}).Extract()
// 	if err != nil {
// 		return GetConsoleErrorf(request, "failed to create Openstack remote console: %v", err), nil
// 	}
// 	// Enable auto scaling on URL
// 	finalURL := remoteConsole.URL
// 	if !strings.Contains(finalURL, "scale=true") {
// 		finalURL = finalURL + "&scale=true"
// 	}

// 	// Return the URL
// 	return &pgrpc.GetConsoleReply{
// 		DeploymentId: request.DeploymentId,
// 		Status:       commonGRPC.RPCStatus_SUCCESS,
// 		Console:      finalURL,
// 		Error:       []string{},
// 	}, nil
// }
