package store

import (
	"net/netip"

	"github.com/caldog20/calnet/control"
	"github.com/caldog20/calnet/types"
)

type Store interface {
	GetNodes() (types.Nodes, error)
	GetPeersOfNode(id uint64) (types.Nodes, error)
	GetNodeByPublicKey(publicKey types.PublicKey) (*control.Node, error)
	GetNodeByID(id uint64) (*control.Node, error)
	CreateNode(node *control.Node) error
	DeleteNode(publicKey types.PublicKey) error
	GetAllocatedIps() ([]netip.Addr, error)
	UpdateNode(node *control.Node) error
	Close() error
}
