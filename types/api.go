package types

import (
	"encoding/json"
	"net/netip"
)

type LoginRequest struct {
	Hostname     string `json:"hostname"`
	ProvisionKey string `json:"provision_key"`
}

type LoginResponse struct {
	AuthURL    string     `json:"auth_url"`
	NodeConfig NodeConfig `json:"node_config"`
}

type ServerKeyResponse struct {
	ServerKey string `json:"server_key"`
}

type NodeUpdateRequest struct {
	Endpoints []netip.AddrPort `json:"endpoints"`
	Hostname  string           `json:"hostname"`
	CallPeer  *CallPeerRequest `json:"call_peer,omitempty"`
}

type NodeUpdateResponse struct {
	Peers        []RemotePeer     `json:"peers,omitempty"`
	NodeConfig   *NodeConfig      `json:"node_config,omitempty"`
	RevokedPeers []RemotePeer     `json:"revoked_peers,omitempty"`
	CallPeer     *CallPeerRequest `json:"call_peer,omitempty"`
}

func (req *NodeUpdateResponse) String() string {
	if req == nil {
		return ""
	}
	b, _ := json.Marshal(req)
	return string(b)
}

type NodeConfig struct {
	ID       uint64         `json:"id"`
	Routes   []netip.Prefix `json:"routes,omitempty"`
	TunnelIP netip.Addr     `json:"tunnel_ip"`
}

type RemotePeer struct {
	ID        uint64           `json:"id"`
	Hostname  string           `json:"hostname"`
	PublicKey PublicKey        `json:"public_key"`
	TunnelIP  netip.Addr       `json:"tunnel_ip"`
	Connected bool             `json:"connected"`
	Endpoints []netip.AddrPort `json:"endpoints"`
}

type CallPeerRequest struct {
	ID        uint64
	Endpoints []netip.AddrPort
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
