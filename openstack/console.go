package openstack

import (
	"context"
	"fmt"
	"strings"

	commonGRPC "github.com/cble-platform/cble-provider-grpc/pkg/common"
	providerGRPC "github.com/cble-platform/cble-provider-grpc/pkg/provider"
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/remoteconsoles"
	"github.com/sirupsen/logrus"
)

func GetConsoleErrorf(request *providerGRPC.GetConsoleRequest, format string, a ...any) *providerGRPC.GetConsoleReply {
	return &providerGRPC.GetConsoleReply{
		DeploymentId: request.DeploymentId,
		Status:       commonGRPC.RPCStatus_FAILURE,
		Console:      "",
		Errors: []string{
			fmt.Sprintf(format, a...),
		},
	}
}

func (provider ProviderOpenstack) GetConsole(ctx context.Context, request *providerGRPC.GetConsoleRequest) (*providerGRPC.GetConsoleReply, error) {
	logrus.Debugf("Deploy called for deployment \"%s\"", request.DeploymentId)

	// Check if the provider has been configured
	if CONFIG == nil {
		return nil, fmt.Errorf("cannot deploy with unconfigured provider, please call Configure()")
	}

	// Generate authenticated client session
	authClient, err := provider.newAuthClient()
	if err != nil {
		return nil, fmt.Errorf("failed to authenticate: %v", err)
	}

	// Convert protobuf maps to go maps
	deploymentVars := request.DeploymentVars.AsMap()
	deploymentState := request.DeploymentState.AsMap()

	// Check that the host state is good
	if deploymentState[request.HostKey] != commonGRPC.DeployStateSUCCEEDED {
		return GetConsoleErrorf(request, "host %s is not in SUCCEEDED state", request.HostKey), nil
	}

	// Check for VM ID in vars
	instanceId, ok := deploymentVars[request.HostKey+"_id"]
	if !ok {
		return GetConsoleErrorf(request, "host %s does not have instance id", request.HostKey), nil
	}
	// Ensure instanceId is string
	instanceIdStr, ok := instanceId.(string)
	if !ok {
		return GetConsoleErrorf(request, "host %s has non-string id", request.HostKey), nil
	}

	// Generate the Compute V2 client
	endpointOpts := gophercloud.EndpointOpts{
		Region: CONFIG.RegionName,
	}
	computeClient, err := openstack.NewComputeV2(authClient, endpointOpts)
	if err != nil {
		return GetConsoleErrorf(request, "failed to create compute v2 client: %v", err), nil
	}
	// Set the microversion of the compute api (min for remote consoles is 2.6)
	computeClient.Microversion = "2.6"

	// Create the remote console and return the URL
	remoteConsole, err := remoteconsoles.Create(computeClient, instanceIdStr, remoteconsoles.CreateOpts{
		Protocol: CONFIG.PreferredConsoleProtocol,
		Type:     CONFIG.PreferredConsoleType,
	}).Extract()
	if err != nil {
		return GetConsoleErrorf(request, "failed to create Openstack remote console: %v", err), nil
	}
	// Enable auto scaling on URL
	finalURL := remoteConsole.URL
	if !strings.Contains(finalURL, "scale=true") {
		finalURL = finalURL + "&scale=true"
	}

	// Return the URL
	return &providerGRPC.GetConsoleReply{
		DeploymentId: request.DeploymentId,
		Status:       commonGRPC.RPCStatus_SUCCESS,
		Console:      finalURL,
		Errors:       []string{},
	}, nil
}
