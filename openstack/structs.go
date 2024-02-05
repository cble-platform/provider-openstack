package openstack

import (
	"fmt"
	"net"
	"net/netip"

	"github.com/cble-platform/cble-backend/engine/models"
	"gopkg.in/yaml.v3"
)

type OpenstackResourceType string

const (
	OpenstackResourceTypeHost    OpenstackResourceType = "openstack.v1.host"
	OpenstackResourceTypeNetwork OpenstackResourceType = "openstack.v1.network"
	OpenstackResourceTypeRouter  OpenstackResourceType = "openstack.v1.router"
)

type OpenstackBlueprint struct {
	// Inherit standard object values
	models.Blueprint `yaml:",inline"`
	// Openstack specific values
	Objects  map[string]OpenstackObject  `yaml:",inline"`
	Hosts    map[string]OpenstackHost    `yaml:"-"`
	Networks map[string]OpenstackNetwork `yaml:"-"`
	Routers  map[string]OpenstackRouter  `yaml:"-"`
}

type OpenstackObject struct {
	// Inherit standard object values
	models.Object `yaml:",inline"`
	// Openstack specific values
	Data     *OpenstackResourceType `yaml:"-"`
	Resource *OpenstackResourceType `yaml:"-"`
	Host     *OpenstackHost         `yaml:"-"`
	Network  *OpenstackNetwork      `yaml:"-"`
	Router   *OpenstackRouter       `yaml:"-"`
}

func (o *OpenstackObject) UnmarshalYAML(n *yaml.Node) error {
	type O OpenstackObject
	type T struct {
		*O `yaml:",inline"`
	}

	obj := &T{O: (*O)(o)}
	if err := n.Decode(obj); err != nil {
		return err
	}

	// Convert resource string into openstack resource type
	var t OpenstackResourceType
	if o.Object.Resource != nil {
		t = OpenstackResourceType(*o.Object.Resource)
		o.Resource = &t
	} else if o.Object.Data != nil {
		t = OpenstackResourceType(*o.Object.Data)
		o.Data = &t
	}

	// Marshall the various resource types
	switch t {
	case OpenstackResourceTypeHost:
		o.Host = new(OpenstackHost)
		return obj.Config.Decode(o.Host)
	case OpenstackResourceTypeNetwork:
		o.Network = new(OpenstackNetwork)
		return obj.Config.Decode(o.Network)
	case OpenstackResourceTypeRouter:
		o.Router = new(OpenstackRouter)
		return obj.Config.Decode(o.Router)
	default:
		return fmt.Errorf("unknown resource type \"%s\"", t)
	}
}

type OpenstackHost struct {
	// Openstack instance id
	ID *string `yaml:"id,omitempty"`
	// Openstack instance name
	Name *string `yaml:"name,omitempty"`
	// Openstack instance description
	Description *string `yaml:"description,omitempty"`
	// Hostname of the host
	Hostname string `yaml:"hostname,omitempty"`
	// Image of the host
	Image string `yaml:"image,omitempty"`
	// Flavor of the host
	Flavor string `yaml:"flavor,omitempty"`
	// Disk size of the host (in GB)
	DiskSize int `yaml:"disk_size,omitempty"`
	// Networks to attach this host to
	Networks map[string]OpenstackNetworkAttachment `yaml:"networks,omitempty"`
	// Any userdata to pass to created instance
	UserData []byte `yaml:"user_data,omitempty"`
}

type OpenstackNetworkAttachment struct {
	// Should this interface get IP via DHCP (overrides IP setting if set)
	DHCP bool `yaml:"dhcp,omitempty"`
	// IPv4 address to use for the interface
	IP *netip.Addr `yaml:"ip,omitempty"`
}

type OpenstackNetwork struct {
	// Openstack network id
	ID *string `yaml:"id,omitempty"`
	// Openstack network name
	Name *string `yaml:"name,omitempty"`
	// Openstack network description
	Description *string `yaml:"description,omitempty"`
	// The subnet CIDR for the network
	Subnet netip.Prefix `yaml:"subnet"`
	// The gateway for the network
	Gateway *netip.Addr `yaml:"gateway,omitempty"`
	// DHCP ranges for the network (omit to disable DHCP)
	DHCP []OpenstackNetworkDHCP `yaml:"dhcp,omitempty"`
	// DNS servers for the network (omit to disable DNS)
	Resolvers []net.Addr `yaml:"resolvers,omitempty"`
}

type OpenstackNetworkDHCP struct {
	// The start IP address for the DHCP range
	Start netip.Addr `yaml:"start"`
	// The end IP address for the DHCP range
	End netip.Addr `yaml:"end"`
}

type OpenstackRouter struct {
	// Openstack router id
	ID *string `yaml:"id,omitempty"`
	// Openstack router name
	Name *string `yaml:"name,omitempty"`
	// Openstack router description
	Description *string `yaml:"description,omitempty"`
	// The ID or Name of the external Openstack network to attach this router to
	ExternalNetwork string `yaml:"external_network"`
	// Networks to attach this host to
	Networks map[string]OpenstackNetworkAttachment `yaml:"networks"`
}
