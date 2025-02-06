package store

import (
	"fmt"
	"log"
	"net/netip"
	"os"
	"testing"
	"time"

	"github.com/caldog20/calnet/control"
	"github.com/caldog20/calnet/types"
)

var s *SqlStore
var testKey types.PrivateKey

func TestMain(m *testing.M) {
	var err error
	s, err = NewSqlStore(":memory:")
	if err != nil {
		log.Fatal(err)
	}
	testKey = types.NewPrivateKey()
	os.Exit(m.Run())
}

func TestCreateNode(t *testing.T) {
	n := &control.Node{
		PublicKey: testKey.Public(),
		KeyExpiry: time.Now(),
		TunnelIP:  netip.MustParseAddr("1.1.1.1"),
		Hostname:  "TestNode",
		//Endpoints: []netip.AddrPort{netip.MustParseAddrPort("2.2.2.2:2222")},
		Routes: []netip.Prefix{
			netip.MustParsePrefix("10.0.0.0/8"),
		},
	}

	err := s.CreateNode(n)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetNodes(t *testing.T) {
	nodes, err := s.GetNodes()
	if err != nil {
		t.Fatal(err)
	}
	if nodes == nil {
		t.Fatal("got nil nodes")
	}
	fmt.Printf("nodes: %+v\n", nodes[0])
}

func TestGetAllocatedIPs(t *testing.T) {
	ips, err := s.GetAllocatedIps()
	if err != nil {
		t.Fatal(err)
	}
	if ips == nil {
		t.Fatal("got nil ips")
	}
}

func TestSqlStore_GetNodeByPublicKey(t *testing.T) {
	node, err := s.GetNodeByPublicKey(testKey.Public())
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("%+v", node)
}
