package openstack

import (
	providerGRPC "github.com/cble-platform/cble-provider-grpc/pkg/provider"
)

type ProviderOpenstack struct {
	providerGRPC.DefaultProviderServer
}

const (
	name        = "Openstack"
	description = "Builder that interfaces with Openstack"
	author      = "Bradley Harker <github.com/BradHacker>"
	version     = "1.0"
)

var CONFIG *ProviderOpenstackConfig

func (provider *ProviderOpenstack) Name() string {
	return name
}

func (provider *ProviderOpenstack) Description() string {
	return description
}

func (provider *ProviderOpenstack) Author() string {
	return author
}

func (provider *ProviderOpenstack) Version() string {
	return version
}
