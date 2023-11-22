package openstack

import (
	"sync"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"google.golang.org/protobuf/types/known/structpb"
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

func marshalSyncMap(src *sync.Map) (*structpb.Struct, error) {
	// Fill a regular Go map with sync.Map values
	regMap := make(map[string]interface{})
	src.Range(func(key, value any) bool {
		keyVal, ok := key.(string)
		if !ok {
			return false
		}
		regMap[keyVal] = value
		return true
	})
	// Convert to structpb.Struct
	return structpb.NewStruct(regMap)
}
