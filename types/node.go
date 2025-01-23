package types

import (
	"net/netip"
	"time"
)

type Node struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement"`
	PublicKey PublicKey `gorm:"serializer:json"`
	KeyExpiry time.Time
	TunnelIP  netip.Addr `gorm:"serializer:json"`
	Hostname  string
	Connected bool
	Disabled  bool
	Endpoints []netip.AddrPort `gorm:"serializer:json"`
	Routes    []netip.Prefix   `gorm:"serializer:json"`

	CreatedAt time.Time
	UpdatedAt time.Time
	// User string
}

func (p *Node) IsDisabled() bool {
	return p.Disabled
}

func (p *Node) IsExpired() bool {
	return time.Now().After(p.KeyExpiry)
}

func (p *Node) GetEndpoints() []netip.AddrPort {
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
