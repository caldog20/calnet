package types

import (
	"net/netip"
	"time"
)

type Peer struct {
	ID         uint64
	ControlKey string
	PublicKey  string
	KeyExpiry  time.Time
	TunnelIP   netip.Addr
	Hostname   string
	Connected  bool
	Disabled   bool
}
