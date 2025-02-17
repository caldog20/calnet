package node

import (
	"net/netip"
	"time"

	"github.com/caldog20/calnet/pkg/keys"
)

const (
	// TODO: Move key expiry time to config
	DefaultKeyExpiryDays     = 180
	DefaultKeyExpiryDuration = (time.Hour * 24) * DefaultKeyExpiryDays
)

type Node struct {
	ID uint64
	// Implement control plane encryption later
	// ControlKey keys.PublicKey
	NodeKey keys.PublicKey
	// Hostname string
	IP     netip.Addr
	Prefix netip.Prefix
	// For Node Key
	KeyExpiry time.Time

	User          string
	Disabled      bool
	LastConnected time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (n *Node) IsExpired() bool {
	return time.Now().After(n.KeyExpiry)
}

func (n *Node) IsDisabled() bool {
	return n.Disabled
}
