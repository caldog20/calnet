package mux

import (
	"fmt"
	"log"
	"net"
	"net/netip"
	"strings"
)

func getEndpointsForUnspecified(port uint16) ([]netip.AddrPort, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	var eps []netip.AddrPort
	parse := func(ip net.IP) netip.AddrPort {
		a, _ := netip.AddrFromSlice(ip)
		a = a.Unmap()
		return netip.AddrPortFrom(a, port)
	}

	for _, i := range ifaces {
		if i.Flags&net.FlagUp == 0 {
			continue
		}
		if i.Flags&net.FlagLoopback != 0 {
			continue
		}
		if strings.Contains(i.Name, "tun") {
			continue
		}

		addrs, err := i.Addrs()
		if err != nil {
			continue
		}

		for _, a := range addrs {
			switch v := a.(type) {
			case *net.UDPAddr:
				// fmt.Printf("udp %s\n", v.String())

				if v.IP.To4() != nil {
					eps = append(eps, parse(v.IP))
				}
			case *net.IPAddr:
				// fmt.Printf("ipaddr %v : %s (%s)\n", i.Name, v, v.IP.DefaultMask())
				if v.IP.To4() != nil {
					eps = append(eps, parse(v.IP))
				}
			case *net.IPNet:
				// fmt.Printf("net %v : %s [%v/%v]\n", i.Name, v, v.IP, v.Mask)
				if v.IP.To4() != nil {
					eps = append(eps, parse(v.IP))
				}
			default:
				fmt.Printf("unhandled address type %T\n", v)
			}
		}
	}
	return eps, nil
}
