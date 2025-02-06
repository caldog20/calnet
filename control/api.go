package control

import (
	"net/netip"

	"github.com/caldog20/calnet/types"
)

type LoginRequest struct {
	NodeKey      types.PublicKey `json:"node_key"`
	Hostname     string          `json:"hostname"`
	ProvisionKey string          `json:"provision_key"`
}

type LoginResponse struct {
	AuthURL    string     `json:"auth_url"`
	NodeConfig NodeConfig `json:"node_config"`
}

type ServerKeyResponse struct {
	ServerKey types.PublicKey `json:"server_key"`
}

type PollRequest struct {
	NodeKey types.PublicKey `json:"node_key"`
}

type PollResponse struct {
	Peers []RemotePeer `json:"peers,omitempty"`
}

type NodeConfig struct {
	ID       uint64       `json:"id"`
	Prefix   netip.Prefix `json:"prefix"`
	TunnelIP netip.Addr   `json:"tunnel_ip"`
}

type RemotePeer struct {
	ID        uint64          `json:"id"`
	Hostname  string          `json:"hostname"`
	PublicKey types.PublicKey `json:"public_key"`
	TunnelIP  netip.Addr      `json:"tunnel_ip"`
}
