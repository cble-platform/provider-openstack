package openstack

import (
	"context"
	"fmt"

	pgrpc "github.com/cble-platform/cble-provider-grpc/pkg/provider"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/remoteconsoles"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

type ProviderOpenstackConfig struct {
	AuthUrl                  string                         `yaml:"auth_url"`
	Username                 string                         `yaml:"username"`
	Password                 string                         `yaml:"password"`
	ProjectID                string                         `yaml:"project_id"`
	ProjectName              string                         `yaml:"project_name"`
	RegionName               string                         `yaml:"region_name"`
	DomainName               string                         `yaml:"domain_name,omitempty"`
	DomainId                 string                         `yaml:"domain_id,omitempty"`
	PreferredConsoleType     remoteconsoles.ConsoleType     `yaml:"console_type,omitempty"`
	PreferredConsoleProtocol remoteconsoles.ConsoleProtocol `yaml:"console_protocol,omitempty"`
}

func ConfigFromBytes(in []byte) (*ProviderOpenstackConfig, error) {
	var config ProviderOpenstackConfig
	if err := yaml.Unmarshal(in, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %v", err)
	}
	return &config, nil
}

func (provider ProviderOpenstack) Configure(ctx context.Context, request *pgrpc.ConfigureRequest) (*pgrpc.ConfigureReply, error) {
	logrus.Debugf("----- Configure called with %d byte config -----", len(request.Config))

	config, err := ConfigFromBytes(request.Config)
	if err != nil {
		return &pgrpc.ConfigureReply{
			Success: false,
		}, fmt.Errorf("failed to read config: %v", err)
	}

	// Set the provider config
	CONFIG = config

	// Test the connection
	if _, err := provider.newAuthClient(); err != nil {
		return &pgrpc.ConfigureReply{
			Success: false,
		}, fmt.Errorf("connection test failed: %v", err)
	}

	return &pgrpc.ConfigureReply{
		Success: true,
	}, nil
}
