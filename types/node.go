package types

import (
	"net/netip"
	"time"
)

type Node struct {
	ID        uint64
	PublicKey string
	KeyExpiry time.Time
	TunnelIP  netip.Addr
	Hostname  string
	Connected bool
	Disabled  bool
	Endpoints Endpoints
	Routes    Routes
}

func (p *Node) IsDisabled() bool {
	return p.Disabled
}

func (p *Node) IsExpired() bool {
	return time.Now().After(p.KeyExpiry)
}

func (p *Node) GetEndpoints() Endpoints {
	return p.Endpoints
}

func (p *Node) ToRemotePeer(connected bool) RemotePeer {
	if p != nil {
		return RemotePeer{
			ID:        p.ID,
			Hostname:  p.Hostname,
			PublicKey: p.PublicKey,
			TunnelIP:  p.TunnelIP,
			Endpoints: p.Endpoints,
			Connected: connected,
		}
	}
	return RemotePeer{}
}

type Nodes []*Node

func (p Nodes) Len() int { return len(p) }
func (p Nodes) ToMap() map[uint64]*Node {
	m := make(map[uint64]*Node)
	for _, node := range p {
		m[node.ID] = node
	}
	return m
}
