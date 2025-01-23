package node

import (
	"github.com/caldog20/calnet/types"
)

type Node struct {
	ID uint64
	// TODO: Logic to update keys
	PrivateKey types.PrivateKey
	PublicKey  types.PublicKey

	Client *Client

	// tunnel configuration

	// Peers
}
