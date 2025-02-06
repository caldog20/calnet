package probe

import (
	"encoding/binary"
	"encoding/json"
	"math/rand"
	"net/netip"
)

type ProbeType byte

const (
	Magic            uint32 = 0xdeadbeef
	Ping                    = ProbeType(0xA)
	Pong                    = ProbeType(0xB)
	EndpointRequest         = ProbeType(0xC)
	EndpointResponse        = ProbeType(0xD)
  Call = ProbeType(0xE)
)

type Probe struct {
	NodeID   uint64
	TxID     uint64
	Type     ProbeType
	Endpoint *netip.AddrPort
  Endpoints []netip.AddrPort
}

func New(nodeID uint64, pt ProbeType, ep *netip.AddrPort) *Probe {
	p := &Probe{
		NodeID:   nodeID,
		Type:     pt,
		Endpoint: ep,
		TxID:     rand.Uint64(),
	}
	return p
}

func IsProbeMessage(b []byte) bool {
	if len(b) < 4 {
		return false
	}

	magic := binary.BigEndian.Uint32(b[:4])
	if magic != Magic {
		return false
	}

	return true
}

func (p *Probe) Decode(b []byte) error {
	if p == nil {
		panic("cannot decode into nil probe message")
	}

	return json.Unmarshal(b[4:], p)
}

func (p *Probe) Encode() ([]byte, error) {
	js, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}

	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, Magic)
	b = append(b, js...)
	return b, nil
}

func Encode[T any](v T) ([]byte, error) {
	js, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, Magic)
	b = append(b, js...)
	return b, nil
}

func Decode[T any](b []byte, v T) error {
	err := json.Unmarshal(b[4:], v)
	if err != nil {
		return err
	}
	return nil
}
