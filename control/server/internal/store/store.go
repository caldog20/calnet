package store

import (
	"net/netip"

	"github.com/caldog20/calnet/control/server/internal/node"
	"github.com/caldog20/calnet/pkg/keys"
)

type Store interface {
	GetNodes() ([]node.Node, error)
	GetPeersOfNode(id uint64) ([]*node.Node, error)
	GetNodeByKey(key keys.PublicKey) (*node.Node, error)
	GetNodeByID(id uint64) (*node.Node, error)
	CreateNode(node *node.Node) error
	DeleteNode(id uint64) error
	UpdateNode(node *node.Node) error
	GetAllocatedNodeIPs() ([]netip.Addr, error)
}
