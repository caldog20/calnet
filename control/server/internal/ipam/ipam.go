package ipam

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

func NewIPAM(prefix netip.Prefix, allocated []netip.Addr) *IPAM {
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
	}
}

func (i *IPAM) GetPrefix() netip.Prefix {
	return i.prefix
}

func (i *IPAM) Allocate() (netip.Addr, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	ipset, err := i.allocatedIPs.IPSet()
	if err != nil {
		return netip.Addr{}, err
	}

	next := i.last.Next()
	for ipset.Contains(next) {
		next = next.Next()
	}

	if !i.prefix.Contains(next) || i.isReservedIP(next) || !next.IsValid() {
		return netip.Addr{}, ErrNoAvailableIps
	}

	i.last = next
	i.allocatedIPs.Add(next)
	return next, nil
}

func (i *IPAM) Release(ip netip.Addr) {
	i.mu.Lock()
	defer i.mu.Unlock()

	// Remove the provided Addr from the list and reset i.last to prefix.Addr()
	// The next allocation will take a bit longer, but we can recoop the released IP this way
	i.last = i.prefix.Addr()
	i.allocatedIPs.Remove(ip)
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
