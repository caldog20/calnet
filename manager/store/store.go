package store

import (
	"net/netip"

	"github.com/caldog20/calnet/types"
)

type Store interface {
	GetNodes() (types.Nodes, error)
	GetPeersOfNode(id uint64) (types.Nodes, error)
	GetNodeByPublicKey(publicKey types.PublicKey) (*types.Node, error)
	GetNodeByID(id uint64) (*types.Node, error)
	CreateNode(node *types.Node) error
	DeleteNode(publicKey types.PublicKey) error
	GetAllocatedIps() ([]netip.Addr, error)
	UpdateNode(node *types.Node) error
	Close() error
}
