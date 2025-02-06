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

	"github.com/caldog20/calnet/types"
	"github.com/caldog20/calnet/types/probe"
	"github.com/pion/stun"
)

// Currently Mux only supports IPv4 transport

const (
	Udp            = "udp4"
	StunTimer      = time.Second * 10
	StunServerAddr = "stun.l.google.com:19302"
	MaxMTU         = 1400
)

type Mux struct {
	nodeID          uint64
	nodeKey         types.PublicKey
	conn            net.PacketConn
	relayAddr       netip.AddrPort
	listenEndpoints []netip.AddrPort
	discoveredAddr  netip.AddrPort
	xorMappedAddr   stun.XORMappedAddress
	relayClient     *RelayClient
	// Configurable Options
	// stunServers []*net.UDPAddr

	mu        sync.Mutex
	nodeKeys  map[types.PublicKey]*Conn
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

func NewConnMux(
	nodeID uint64,
	nodeKey types.PublicKey,
	conn net.PacketConn,
	relayAddr string,
) *Mux {
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
		nodeKey:         nodeKey,
		listenEndpoints: listenEndpoints,
		mu:              sync.Mutex{},
		nodes:           make(map[uint64]*Conn),
		nodeKeys:        make(map[types.PublicKey]*Conn),
		endpoints:       make(map[netip.AddrPort]*Conn),
		close:           make(chan struct{}),
		closed:          atomic.Bool{},
	}

	mux.stunFunc = time.AfterFunc(StunTimer, mux.doStun)

	mux.relayClient = &RelayClient{
		DialAddr: relayAddr,
		NodeKey:  nodeKey,
	}

	err := mux.relayClient.Dial()
	if err != nil {
		log.Fatal(err)
	}

	go mux.relayRead()
	go mux.stun()
	go mux.read()

	return mux
}

func (mux *Mux) GetConn(nodeID uint64, nodeKey types.PublicKey) (*Conn, error) {
	if mux.IsClosed() {
		return nil, net.ErrClosed
	}

	mux.mu.Lock()
	defer mux.mu.Unlock()
	// TODO: Remove IDs and only use public keys in map
	ec, ok := mux.nodes[nodeID]
	if ok {
		if !ec.IsClosed() {
			return ec, nil
		}
	}
	// Conn is either closed or doesn't exist, so create...
	conn := newConn(mux, nodeKey, nodeID)
	mux.nodes[nodeID] = conn
	mux.nodeKeys[nodeKey] = conn

	return conn, nil
}

func (mux *Mux) RemoveConn(nodeKey types.PublicKey) {
	mux.mu.Lock()
	ec, ok := mux.nodeKeys[nodeKey]
	if ok {
		ec.Close()
		delete(mux.nodeKeys, nodeKey)
		delete(mux.nodes, ec.peerID)
		for c, e := range mux.endpoints {
			if e == ec {
				delete(mux.endpoints, c)
			}
		}
	}
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

func (mux *Mux) relayRead() {
	// defer func() {
	// 	mux.relayClient.Close()
	// 	log.Println("breaking from relay read loop")
	// }()
	for {
		if mux.IsClosed() {
			return
		}
		data, err := mux.relayClient.Read()
		if err != nil {
			log.Println("error reading from relay:", err)
			continue
		}
		if len(data) < 32 {
			continue
		}
		dstKey := types.PublicKeyFromRawBytes(data[:32])
		packet := data[32:]
		if probe.IsProbeMessage(packet) {
			mux.handleProbeFromRelay(dstKey, packet)
			continue
		}

		mux.mu.Lock()
		dstConn, ok := mux.nodeKeys[dstKey]
		mux.mu.Unlock()
		if !ok {
			continue
		}
		dstConn.receive(data[32:])

	}
}

func (mux *Mux) RelayPacket(packet []byte) {
	if mux.IsClosed() {
		return
	}
	mux.mu.Lock()
	defer mux.mu.Unlock()
	err := mux.relayClient.Write(packet)
	if err != nil {
		log.Println("error writing packet to relay:", err)
	}
}

func (mux *Mux) stun() {
	mux.mu.Lock()
	defer mux.mu.Unlock()
	mux.stunFunc.Stop()
	mux.doStun()
}

func (mux *Mux) doStun() {
	if mux.IsClosed() {
		return
	}
	defer mux.stunFunc.Reset(StunTimer)

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

func (mux *Mux) handleProbeFromRelay(dst types.PublicKey, b []byte) {
	pm := &probe.Probe{}
	err := pm.Decode(b)
	if err != nil {
		log.Println("error decoding probe from relay")
		return
	}

	mux.mu.Lock()
	conn, ok := mux.nodeKeys[dst]
	mux.mu.Unlock()
	if !ok {
		return
	}

	switch pm.Type {
	case probe.Call:
		conn.handleCall()
	case probe.EndpointRequest:
		conn.handleEndpointRequest(pm.Endpoints)
	case probe.EndpointResponse:
		conn.handleEndpointResponse(pm.Endpoints)
	default:
		log.Println("received unexpected probe type from relay:", pm.Type)
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
	case probe.Ping:
		conn.addCandidateFromPing(ep)
		mux.pingReply(pm.TxID, ep)
	// Probe Pong - Pass to conn to handle
	case probe.Pong:
		// Got pong message, tell conn
		conn.handlePong(pm, ep)
		// Not really using right now
		// case probe.ProbeSelect:
		// conn.handleSelect(pm, ep)
	}
}

func (mux *Mux) pingReply(txid uint64, ep netip.AddrPort) {
	pm := &probe.Probe{
		Type:     probe.Pong,
		TxID:     txid,
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
	if mux.IsClosed() {
		return net.ErrClosed
	}
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
		return []netip.AddrPort{netip.MustParseAddrPort(mux.LocalAddr().String())}
	}
	return mux.listenEndpoints
}

func (mux *Mux) GetEndpoints() []netip.AddrPort {
	addrs := mux.ListenAddresses()
	stunAddr := mux.XorMappedAddress()
	if stunAddr.IsValid() {
		addrs = append(addrs, mux.XorMappedAddress())
	}
	return addrs
}

func (mux *Mux) LocalAddr() net.Addr {
	return mux.conn.LocalAddr()
}

// Once mux is closed, the state is invalid and it cannot be reused
func (mux *Mux) Close() error {
	if mux.closed.Load() {
		return nil
	}
	log.Println("Closing Mux")

	mux.closed.Store(true)
	mux.relayClient.Close()
	mux.stunFunc.Stop()
	log.Println("locking mux")

	mux.mu.Lock()
	defer mux.mu.Unlock()
	for _, conn := range mux.nodes {
		conn.Close()
	}

	clear(mux.nodes)
	clear(mux.endpoints)

	mux.conn.Close()
	log.Println("mux closed")
	return nil
}

func (mux *Mux) IsClosed() bool {
	return mux.closed.Load()
}
