package openstack

import (
	"fmt"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
)

func (provider ProviderOpenstack) newAuthClient() (*gophercloud.ProviderClient, error) {
	authOpts := gophercloud.AuthOptions{
		IdentityEndpoint: CONFIG.AuthUrl,
		Username:         CONFIG.Username,
		Password:         CONFIG.Password,
		TenantID:         CONFIG.ProjectID,
		TenantName:       CONFIG.ProjectName,
	}
	if CONFIG.DomainName != "" {
		authOpts.DomainName = CONFIG.DomainName
	} else {
		authOpts.DomainID = CONFIG.DomainId
	}
	return openstack.AuthenticatedClient(authOpts)
}

func Errorf(format string, a ...any) *string {
	err := fmt.Errorf(format, a...).Error()
	return &err
}
