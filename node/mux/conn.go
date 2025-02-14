package mux

import (
	"fmt"
	"log"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/caldog20/calnet/types"
	"github.com/caldog20/calnet/types/probe"
)

const (
	RecheckBestAddr        = time.Millisecond * 5500
	EndpointPingInterval = time.Second * 5
)

var InvalidAddress = netip.AddrPort{}

type sentping struct {
	txID   uint64
	sentAt time.Time
	ep     netip.AddrPort
}

type Endpoint struct {
	addr     netip.AddrPort
	lastPing time.Time
	// last time we got a ping response from this endpoint
	// use IsZero() to check if we ever got a pong
	// On the first pong, send back a ping to ensure bidirectional connectivity is possible
	lastPong time.Time
	// measured latency
	rtt         time.Duration
	activeSince time.Time
}

type Conn struct {
	peerID    uint64
	publicKey types.PublicKey
	mux       *Mux

	mu                 sync.Mutex
	candidates         []netip.AddrPort
	sendAddr           netip.AddrPort
	recheckBest        time.Time
	lastPacketReceived time.Time
	lastPacketSent     time.Time
	lastExchange time.Time
	pings        map[uint64]sentping
	endpoints    map[netip.AddrPort]*Endpoint
	closed       atomic.Bool
}

func newConn(mux *Mux, key types.PublicKey, peerID uint64) *Conn {
	c := &Conn{
		mux:        mux,
		peerID:     peerID,
		publicKey:  key,
		mu:         sync.Mutex{},
		endpoints:  make(map[netip.AddrPort]*Endpoint),
		pings:      make(map[uint64]sentping),
		candidates: []netip.AddrPort{},
		closed:     atomic.Bool{},
	}

	return c
}

func (c *Conn) pingAllLocked() {
	if len(c.endpoints) == 0 || time.Since(c.lastExchange).Seconds() > 120 {
		c.exchange()
		return
	}

	for _, ep := range c.endpoints {
		if time.Since(ep.lastPing) <= EndpointPingInterval {
			continue
		}

		if time.Since(ep.activeSince).Seconds() > 45 {
			if ep.lastPong.IsZero() || time.Since(ep.lastPong).Seconds() > 10 {
				c.deleteEndpointLocked(ep)
        if c.sendAddr == ep.addr {
          c.sendAddr = InvalidAddress
        }
			}
		}

		err := c.pingLocked(ep)
		if err != nil {
			log.Printf("ping failed: %v", err)
			continue
		}
	}
}

func (c *Conn) pingLocked(ep *Endpoint) error {
	ping := probe.New(c.mux.nodeID, probe.Ping, nil)
	sent := sentping{
		txID:   ping.TxID,
		ep:     ep.addr,
		sentAt: time.Now(),
	}

	out, err := ping.Encode()
	if err != nil {
		return fmt.Errorf("error encoding ping probe: %v", err)
	}

	c.pings[ping.TxID] = sent

	err = c.mux.WriteTo(out, ep.addr)
	if err != nil {
		// Drop the ping since it didn't send
		delete(c.pings, ping.TxID)
		return err
	}

	ep.lastPing = time.Now()

	log.Printf("sent ping txid %d to: %s", ping.TxID, ep.addr.String())
	return nil
}

func (c *Conn) handlePong(pm *probe.Probe, ep netip.AddrPort) {
	if c.IsClosed() {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	sp, ok := c.pings[pm.TxID]
	if !ok {
		log.Printf("received pong from ping we didn't send: txid: %d, ep: %s", pm.TxID, ep.String())
		return
	}

	delete(c.pings, pm.TxID)
	// Calculate round-trip time
	rxTime := time.Now()
	rtt := rxTime.Sub(sp.sentAt)

	if ep != sp.ep {
		// We received a pong from a different address than we sent the ping to, add it
		log.Printf("sent ping to %s - received pong from %s", sp.ep.String(), ep.String())
	}

	endpoint, ok := c.endpoints[ep]
	if !ok {
		log.Printf("endpoint not found: %s", ep.String())
		return
	}

	log.Println("got pong from:", ep.String())

	endpoint.lastPong = rxTime
	endpoint.rtt = rtt
  
  if ep == c.sendAddr {
    return
  }

	var maybeBetterAddr netip.AddrPort
	if !c.sendAddr.IsValid() {
		maybeBetterAddr = endpoint.addr
	} else if !c.sendAddr.Addr().IsPrivate() && endpoint.addr.Addr().IsPrivate() {
		maybeBetterAddr = endpoint.addr
	} else {
		curEp, ok := c.endpoints[c.sendAddr]
		if ok {
			if curEp.rtt > endpoint.rtt && endpoint.rtt > 0 {
				maybeBetterAddr = endpoint.addr
			}
		}
	}

	if maybeBetterAddr.IsValid() && maybeBetterAddr != c.sendAddr {
		fmt.Printf(
			"maybe have better address - old: %s - new: %s",
			c.sendAddr.String(),
			maybeBetterAddr.String(),
		)
		c.sendAddr = maybeBetterAddr
		c.recheckBest = time.Now().Add(time.Second * 2)
		// TODO Make this method on mux
		// Map address to mux for future data packets from this endpoint
		c.mux.mu.Lock()
		c.mux.endpoints[endpoint.addr] = c
		c.mux.mu.Unlock()
	}
}

func (c *Conn) deleteEndpointLocked(ep *Endpoint) {
	delete(c.endpoints, ep.addr)
}

func (c *Conn) getBestAddr() netip.AddrPort {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.sendAddr.IsValid() {
		c.pingAllLocked()
		return netip.AddrPort{}
	}

	if c.recheckBest.IsZero() || time.Since(c.recheckBest).Milliseconds() > 5500 {
    c.pingAllLocked()
	}

	if !c.sendAddr.IsValid() {
		fmt.Println("best address is relay")
	} else {
		fmt.Printf("best address is %s\n", c.sendAddr.String())
	}

	return c.sendAddr
}

// TODO: Return error incase of closure
func (c *Conn) Write(bytes []byte) {
	if c.IsClosed() {
		return
	}
	sendAddr := c.getBestAddr()
	if !sendAddr.IsValid() {
		log.Printf("sending using relay")
		c.writeToRelay(bytes)
		return
	}

	c.mux.WriteTo(bytes, sendAddr)
}

func (c *Conn) writeToRelay(data []byte) {
	// TODO: This will need locking if we add ability to change public key for this node
	// without re-creating the conn
	if c.IsClosed() {
		return
	}
	dstKey := c.publicKey.Raw()
	packet := append(dstKey, data...)
	c.mux.RelayPacket(packet)
}

func (c *Conn) receive(bytes []byte) {
	if c.IsClosed() {
		return
	}
	fmt.Println("received from peer", string(bytes))
	// time.Sleep(time.Millisecond * 300)
	c.Write(bytes)
}

func (c *Conn) addCandidateEndpointsLocked(eps []netip.AddrPort) {
	fmt.Println("adding candidate endpoints", eps)
	// atleastOne := false
	for _, ep := range eps {
		if !ep.IsValid() {
			continue
		}
		existing, ok := c.endpoints[ep]
		if !ok {
			c.endpoints[ep] = &Endpoint{
				addr:        ep,
				activeSince: time.Now(),
			}
		} else {
			existing.activeSince = time.Now()
		}
	}
}

func (c *Conn) addCandidateFromPing(ipp netip.AddrPort) {
	if c.IsClosed() {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.addCandidateEndpointsLocked([]netip.AddrPort{ipp})
}

func (c *Conn) exchange() {
	log.Println("exchange triggered")
	// Restun and wait a bit to try to get a fresh binding
	c.mux.stun()
	time.Sleep(time.Millisecond * 50)

	endpoints := c.mux.GetEndpoints()
	epRequest := &probe.Probe{Type: probe.EndpointRequest, Endpoints: endpoints}
	msg, err := epRequest.Encode()
	if err != nil {
		log.Println("error encoding endpoint response")
		return
	}
	c.writeToRelay(msg)
	c.lastExchange = time.Now()
}

func (c *Conn) handleEndpointRequest(endpoints []netip.AddrPort) {
	if c.IsClosed() {
		return
	}

	c.mux.stun()
	time.Sleep(time.Millisecond * 50)
	log.Println("got endpoint request")
	if len(endpoints) == 0 {
		return
	}
	// Add requesting peers endpoints ahead of time
	c.mu.Lock()
	c.addCandidateEndpointsLocked(endpoints)
	c.lastExchange = time.Now()
	c.mu.Unlock()
	// reply to peer with our endpoints
	ourEndpoints := c.mux.GetEndpoints()

	epResponse := &probe.Probe{Type: probe.EndpointResponse, Endpoints: ourEndpoints}
	msg, err := probe.Encode(epResponse)
	if err != nil {
		log.Println("error encoding endpoint response")
		return
	}

	c.writeToRelay(msg)
}

func (c *Conn) handleEndpointResponse(endpoints []netip.AddrPort) {
	if c.IsClosed() {
		return
	}
	log.Println("got endpoint response")
	c.mu.Lock()
	c.addCandidateEndpointsLocked(endpoints)
	c.mu.Unlock()
}

func (c *Conn) requestCall() {
	log.Println("requesting call")
	call := probe.New(0, probe.Call, nil)
	msg, err := call.Encode()
	if err != nil {
		log.Println("error encoding call request")
		return
	}
	c.writeToRelay(msg)
}

func (c *Conn) handleCall() {
	log.Println("got call request")
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pingAllLocked()
}

func (c *Conn) IsClosed() bool {
	return c.closed.Load()
}

func (c *Conn) Close() {
	if c.closed.Load() {
		return
	}

	c.closed.Store(true)
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, ep := range c.endpoints {
		c.deleteEndpointLocked(ep)
	}
}
