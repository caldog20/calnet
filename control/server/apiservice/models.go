package apiservice

import (
	"net/netip"
	"time"

	"github.com/caldog20/calnet/pkg/keys"
)

type Node struct {
	ID        uint64         `json:"id"`
	IP        netip.Addr     `json:"ip_address"`
	NetPrefix netip.Prefix   `json:"net_prefix"`
	
	NodeKey   keys.PublicKey `json:"node_key"`
	KeyExpiry time.Time      `json:"key_expiry"`
	
	User      string         `json:"user"`
	Disabled  bool           `json:"disabled"`
	
	LastSeen  time.Time      `json:"last_seen"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type Nodes struct {
	Nodes []Node `json:"nodes"`
}
