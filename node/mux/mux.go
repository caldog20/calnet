package mux

import (
	"errors"
	"io"
	"log"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/caldog20/calnet/node/probe"
	"github.com/pion/stun"
)

// Currently Mux only supports IPv4 transport

const (
	Udp            = "udp4"
	StunTimer      = time.Second * 10
	StunServerAddr = "stun:stun.l.google.com:19302"
	MaxMTU         = 1400
)

type Mux struct {
	nodeID uint64
	conn   net.PacketConn

	listenEndpoints []netip.AddrPort
	discoveredAddr  netip.AddrPort
	xorMappedAddr   stun.XORMappedAddress

	// Configurable Options
	//stunServers []*net.UDPAddr

	mu        sync.Mutex
	nodes     map[uint64]*Conn
	endpoints map[netip.AddrPort]*Conn

	close  chan struct{}
	closed atomic.Bool

	stunFunc *time.Timer

	counters struct {
		conns        atomic.Uint64
		rxBytes      atomic.Uint64
		txBytes      atomic.Uint64
		stunRequests atomic.Uint64
		cacheHits    atomic.Uint64
	}
}

func NewConnMux(nodeID uint64, conn net.PacketConn) *Mux {
	var listenEndpoints []netip.AddrPort
	if listenAddr, ok := conn.LocalAddr().(*net.UDPAddr); ok {
		if listenAddr.IP.IsUnspecified() {
			endpoints, err := getEndpointsForUnspecified(uint16(listenAddr.Port))
			if err != nil {
				log.Printf("Error getting endpoints for unspecified port: %v", err)
			} else {
				listenEndpoints = endpoints
			}
		}
	} else {
		panic("mux: conn is not a UDP connection")
	}

	mux := &Mux{
		conn:            conn,
		nodeID:          nodeID,
		listenEndpoints: listenEndpoints,
		mu:              sync.Mutex{},
		nodes:           make(map[uint64]*Conn),
		endpoints:       make(map[netip.AddrPort]*Conn),
		close:           make(chan struct{}),
		closed:          atomic.Bool{},
	}

	go mux.stun()
	go mux.read()

	return mux
}

func (mux *Mux) read() {
	defer mux.Close()

	buf := make([]byte, MaxMTU)

	var cachedEp netip.AddrPort
	var cachedConn *Conn

	for {
		n, addr, err := mux.conn.ReadFrom(buf)
		if err != nil {
			if mux.IsClosed() {
				return
			}
			if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
				return
			}
			log.Printf("Error reading from UDP connection: %v", err)
			return
		}

		if stun.IsMessage(buf[:n]) {
			msg := &stun.Message{
				Raw: append([]byte{}, buf[:n]...),
			}
			mux.handleStun(msg)
			continue
		}

		ep := addr.(*net.UDPAddr).AddrPort()

		if probe.IsProbeMessage(buf[:n]) {
			mux.handleProbe(buf[:n], ep)
			continue
		}

		if ep == cachedEp && cachedConn != nil {
			mux.counters.cacheHits.Add(1)
			cachedConn.receive(buf[:n])
			continue
		}

		mux.mu.Lock()
		conn, ok := mux.endpoints[ep]
		mux.mu.Unlock()

		if !ok {
			log.Println("Unknown endpoint:", ep)
			continue
		}

		// Sanity Check
		if conn == nil {
			panic("mux: conn is nil for ep")
		}

		cachedEp = ep
		cachedConn = conn

		conn.receive(buf[:n])
	}
}

func (mux *Mux) stun() {
	mux.doStun()
	mux.stunFunc = time.AfterFunc(StunTimer, mux.stun)
}

func (mux *Mux) doStun() {
	if mux.IsClosed() {
		return
	}

	stunAddr, err := net.ResolveUDPAddr(Udp, StunServerAddr)
	if err != nil {
		log.Printf("error sending stun request")
		return
	}

	msg := stun.MustBuild(stun.TransactionID, stun.BindingRequest)
	_, err = mux.conn.WriteTo(msg.Raw, stunAddr)
	if err != nil {
		log.Printf("error sending stun request")
		return
	}
}

func (mux *Mux) handleProbe(b []byte, ep netip.AddrPort) {
	pm := &probe.Probe{}
	err := pm.Decode(b)
	if err != nil {
		log.Printf("error decoding probe message: %s", err)
		return
	}

	mux.mu.Lock()
	conn, ok := mux.nodes[pm.NodeID]
	mux.mu.Unlock()

	if !ok {
		log.Printf("received probe message for unknown node id %d", pm.NodeID)
		return
	}

	switch pm.Type {
	// Ping Probe - Send Ping back to endpoint
	case probe.ProbePing:
		conn.addCandidateFromPing(ep)
		mux.pingReply(ep)
	// Probe Pong - Pass to conn to handle
	case probe.ProbePong:
		// Got pong message, tell conn
		conn.handlePong(pm, ep)
	// Not really using right now
	case probe.ProbeSelect:
		//conn.handleSelect(pm, ep)
	}
}

func (mux *Mux) pingReply(ep netip.AddrPort) {
	pm := &probe.Probe{
		Type:     probe.ProbePong,
		NodeID:   mux.nodeID,
		Endpoint: &ep,
	}

	out, err := pm.Encode()
	if err != nil {
		log.Println("error encoding probe message:", err)
		return
	}

	if err = mux.WriteTo(out, ep); err != nil {
		log.Printf("error writing probe packet to remote %s: %s", ep.String(), err)
	}
}

func (mux *Mux) handleStun(msg *stun.Message) {
	err := msg.Decode()
	if err != nil {
		log.Printf("error decoding stun message: %v", err)
		return
	}

	if msg.Type != stun.BindingSuccess {
		log.Printf("invalid stun response type: %s", msg.Type.String())
		return
	}

	var xor stun.XORMappedAddress
	err = xor.GetFrom(msg)
	if err != nil {
		log.Printf("error getting xormappedaddr from msg: %v", err)
		return
	}

	if xor.String() == mux.xorMappedAddr.String() {
		return
	}

	mux.mu.Lock()
	defer mux.mu.Unlock()

	stunAddr, err := netip.ParseAddrPort(xor.String())
	if err != nil {
		log.Printf("error parsing stun xor-mapped address: %v", err)
		return
	}

	mux.discoveredAddr = stunAddr
}

func (mux *Mux) WriteTo(b []byte, ep netip.AddrPort) error {
	_, err := mux.conn.WriteTo(b, net.UDPAddrFromAddrPort(ep))
	return err
}

func (mux *Mux) XorMappedAddress() netip.AddrPort {
	mux.mu.Lock()
	defer mux.mu.Unlock()
	return mux.discoveredAddr
}

func (mux *Mux) ListenAddresses() []netip.AddrPort {
	if mux.listenEndpoints == nil || len(mux.listenEndpoints) == 0 {
		return []netip.AddrPort{netip.MustParseAddrPort(mux.conn.LocalAddr().String())}
	}
	return mux.listenEndpoints
}

func (mux *Mux) LocalAddr() net.Addr {
	return mux.conn.LocalAddr()
}

// Once mux is closed, the state is invalid and it cannot be reused
func (mux *Mux) Close() error {
	if mux.closed.Load() {
		return nil
	}

	mux.closed.Store(true)
	mux.stunFunc.Stop()

	mux.mu.Lock()
	defer mux.mu.Unlock()

	for _, conn := range mux.nodes {
		conn.Close()
	}

	clear(mux.nodes)
	clear(mux.endpoints)

	mux.conn.Close()
	return nil
}

func (mux *Mux) IsClosed() bool {
	return mux.closed.Load()
}
