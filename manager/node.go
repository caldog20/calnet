package manager

import (
	"net/netip"
	"time"

	"github.com/caldog20/calnet/types"
)

type Node struct {
	ID        uint64          `gorm:"primaryKey;autoIncrement"`
	PublicKey types.PublicKey `gorm:"serializer:json"`
	KeyExpiry time.Time
	TunnelIP  netip.Addr `gorm:"serializer:json"`
	Hostname  string
	Connected bool
	Disabled  bool
	Routes    []netip.Prefix `gorm:"serializer:json"`

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

func (p *Node) ToRemotePeer(connected bool) types.RemotePeer {
	if p != nil {
		return types.RemotePeer{
			ID:        p.ID,
			Hostname:  p.Hostname,
			PublicKey: p.PublicKey,
			TunnelIP:  p.TunnelIP,
			Connected: connected,
		}
	}
	return types.RemotePeer{}
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
