package manager

import (
	"errors"
	"net/netip"
	"sync"

	"go4.org/netipx"
)

var ErrNoAvailableIps = errors.New("no free ip addresses in prefix available")

type IPAM struct {
	mu sync.Mutex

	prefix       netip.Prefix
	allocatedIPs netipx.IPSetBuilder
	last         netip.Addr
}

func NewIPAM(prefix netip.Prefix, store Store) (*IPAM, error) {
	allocated, err := store.GetAllocatedIps()
	if err != nil {
		return nil, err
	}

	var b netipx.IPSetBuilder
	for _, ip := range allocated {
		if ip.IsValid() {
			b.Add(ip)
		}
	}

	return &IPAM{
		prefix:       prefix,
		allocatedIPs: b,
		last:         prefix.Addr(),
	}, nil
}

func (i *IPAM) Allocate() (netip.Addr, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	ipset, err := i.allocatedIPs.IPSet()
	if err != nil {
		return netip.Addr{}, err
	}

	next := i.last.Next()
	for {
		if ipset.Contains(next) {
			next = next.Next()
		} else {
			break
		}
	}

	if !i.prefix.Contains(next) || i.isReservedIP(next) {
		return netip.Addr{}, ErrNoAvailableIps
	}

	i.last = next
	i.allocatedIPs.Add(next)

	return next, nil
}

func (i *IPAM) isReservedIP(ip netip.Addr) bool {
	if ip == i.prefix.Addr().Prev() {
		return true
	}
	if ip == i.prefix.Addr() {
		return true
	}
	return false
}
