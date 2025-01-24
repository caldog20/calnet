package store

import (
	"net/netip"

	"github.com/caldog20/calnet/manager"
	"github.com/caldog20/calnet/types"
)

type Store interface {
	GetNodes() (manager.Nodes, error)
	GetPeersOfNode(id uint64) (manager.Nodes, error)
	GetNodeByPublicKey(publicKey types.PublicKey) (*manager.Node, error)
	GetNodeByID(id uint64) (*manager.Node, error)
	CreateNode(node *manager.Node) error
	DeleteNode(publicKey types.PublicKey) error
	GetAllocatedIps() ([]netip.Addr, error)
	UpdateNode(node *manager.Node) error
	Close() error
}
