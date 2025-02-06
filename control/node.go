package control

import (
	"net/netip"
	"time"

	"github.com/caldog20/calnet/types"
)

type Node struct {
	ID        uint64
	PublicKey types.PublicKey
	KeyExpiry time.Time
	TunnelIP  netip.Addr
	Prefix    netip.Prefix
	Hostname  string
	User      string
	//LastPoll  time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (p *Node) IsExpired() bool {
	return time.Now().After(p.KeyExpiry)
}

//func (p *Node) IsConnected() bool {
//	return time.Since(p.LastPoll).Seconds() < 120
//}

func (p *Node) ToRemotePeer() RemotePeer {
	return RemotePeer{
		ID:        p.ID,
		PublicKey: p.PublicKey,
		Hostname:  p.Hostname,
		TunnelIP:  p.TunnelIP,
	}
}
