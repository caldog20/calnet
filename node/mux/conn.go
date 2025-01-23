package mux

import (
	"net/netip"
	"sync"
	"sync/atomic"
)

type Conn struct {
	PeerID uint64
	mux    *Mux

	mu         sync.Mutex
	candidates []netip.AddrPort

	closed atomic.Bool
}

func NewConn(mux *Mux) *Conn {
	return &Conn{}
}
