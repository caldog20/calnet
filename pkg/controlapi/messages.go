package controlapi

import (
	"net/netip"

	"github.com/caldog20/calnet/pkg/keys"
)

type ControlKey struct {
	PublicKey keys.PublicKey `json:"control_key"`
}

type LoginRequest struct {
	NodeKey      keys.PublicKey `json:"node_key"`
	ProvisionKey string         `json:"provision_key"`
	Logout       bool           `json:"logout"`
}

type LoginResponse struct {
	LoggedIn   bool `json:"logged_in"`
	KeyExpired bool `json:"key_expired"`
	// AuthURL string `json:"auth_url"`
	// KeyExpiry time.Time `json:"key_expiry`
}

type PollRequest struct {
	NodeKey     keys.PublicKey `json:"node_key"`
	ForceUpdate bool           `json:"force_update"`
}

type PollResponse struct {
	KeyExpired bool        `json:"expired"`
	Peers      []Peer      `json:"peers,omitempty"`
	Config     *NodeConfig `json:"node_config,omitempty"`
}

type NodeConfig struct {
	ID     uint64       `json:"id"`
	IP     netip.Addr   `json:"ip"`
	Prefix netip.Prefix `json:"prefix"`
}

type Peer struct {
	ID        uint64         `json:"id"`
	IP        netip.Addr     `json:"ip"`
	PublicKey keys.PublicKey `json:"public_key"`
}
