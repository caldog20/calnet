package probe

import (
	"encoding/binary"
	"encoding/json"
	"math/rand"
	"net/netip"
)

type ProbeType byte

const (
	ProbeMagic  uint32 = 0xdeadbeef
	ProbePing          = ProbeType(0xA)
	ProbePong          = ProbeType(0xB)
	ProbeSelect        = ProbeType(0xC)
)

type Probe struct {
	NodeID   uint64
	TxID     uint64
	Type     ProbeType
	Endpoint *netip.AddrPort
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
	if magic != ProbeMagic {
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
	binary.BigEndian.PutUint32(b, ProbeMagic)
	b = append(b, js...)
	return b, nil
}
