package types

import (
	"encoding/json"
	"net/netip"
)

const (
	NodeUpdateRequestMessage = iota + 1
)

type LoginRequest struct {
	Hostname     string `json:"hostname"`
	ProvisionKey string `json:"provision_key"`
}

type LoginResponse struct {
	AuthURL string `json:"auth_url"`
}

type ServerKeyResponse struct {
	ServerKey string `json:"server_key"`
}

type NodeUpdateRequest struct {
	Endpoints []Endpoint `json:"endpoints"`
	Hostname  string     `json:"hostname"`
}

type NodeUpdateResponse struct {
	Peers        []RemotePeer `json:"peers,omitempty"`
	NodeConfig   *NodeConfig  `json:"node_config,omitempty"`
	RevokedPeers []RemotePeer `json:"revoked_peers,omitempty"`
}

type NodeConfig struct {
	ID       uint64 `json:"id"`
	Routes   Routes `json:"routes,omitempty"`
	TunnelIP string `json:"tunnel_ip"`
}

type RemotePeer struct {
	ID        uint64     `json:"id"`
	Hostname  string     `json:"hostname"`
	PublicKey string     `json:"public_key"`
	TunnelIP  netip.Addr `json:"tunnel_ip"`
	Connected bool       `json:"connected"`
	Endpoints []Endpoint `json:"endpoints"`
}

type MessageWrapper struct {
	MessageType int    `json:"message_type"`
	Data        []byte `json:"data"`
}

func NewMessage(messageType int, data []byte) ([]byte, error) {
	wrapped := &MessageWrapper{
		MessageType: messageType,
		Data:        data,
	}

	return json.Marshal(wrapped)
}

func ParseMessage(data []byte) (*MessageWrapper, error) {
	wrapped := &MessageWrapper{}
	err := json.Unmarshal(data, wrapped)
	return wrapped, err
}
