package openstack

import (
	pgrpc "github.com/cble-platform/cble-provider-grpc/pkg/provider"
)

type ProviderOpenstack struct {
	pgrpc.DefaultProviderServer
}

const (
	name        = "provider-openstack"
	description = "Builder that interfaces with Openstack"
	author      = "Bradley Harker <github.com/BradHacker>"
	version     = "v0.1.2"
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
