package store

import (
	"net/netip"
	"sync"

	"github.com/caldog20/calnet/control"
	"github.com/caldog20/calnet/types"
)

type MapStore struct {
	mu         sync.Mutex
	nodeIds    map[uint64]*control.Node
	idSequence uint64
}

func NewMapStore() *MapStore {
	return &MapStore{
		mu:      sync.Mutex{},
		nodeIds: make(map[uint64]*control.Node),
	}
}

func copyNode(n1 *control.Node) *control.Node {
	nodeCopy := &control.Node{
		ID:        n1.ID,
		PublicKey: n1.PublicKey,
		KeyExpiry: n1.KeyExpiry,
		Hostname:  n1.Hostname,
		Connected: n1.Connected,
		Disabled:  n1.Disabled,
	}
	nodeCopy.Routes = append(nodeCopy.Routes, n1.Routes...)

	return nodeCopy
}

func (m *MapStore) GetNodes() (types.Nodes, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	nodes := make(types.Nodes, len(m.nodeIds))

	for _, n := range m.nodeIds {
		nodes = append(nodes, copyNode(n))
	}

	return nodes, nil
}

func (m *MapStore) GetPeersOfNode(id uint64) (types.Nodes, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var nodes types.Nodes
	for _, n := range m.nodeIds {
		if n.ID != id {
			nodes = append(nodes, copyNode(n))
		}
	}
	return nodes, nil
}

func (m *MapStore) GetNodeByPublicKey(publicKey types.PublicKey) (*control.Node, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, n := range m.nodeIds {
		if n.PublicKey == publicKey {
			return copyNode(n), nil
		}
	}
	return nil, ErrNotFound
}

func (m *MapStore) GetNodeByID(id uint64) (*control.Node, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, n := range m.nodeIds {
		if n.ID == id {
			return copyNode(n), nil
		}
	}
	return nil, ErrNotFound
}

func (m *MapStore) CreateNode(node *control.Node) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.idSequence++
	node.ID = m.idSequence
	m.nodeIds[node.ID] = node
	return nil
}

func (m *MapStore) DeleteNode(publicKey types.PublicKey) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, n := range m.nodeIds {
		if n.PublicKey == publicKey {
			delete(m.nodeIds, n.ID)
			return nil
		}
	}
	return ErrNotFound
}

func (m *MapStore) GetAllocatedIps() ([]netip.Addr, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	allocated := make([]netip.Addr, len(m.nodeIds))
	for _, n := range m.nodeIds {
		allocated = append(allocated, n.TunnelIP)
	}
	return allocated, nil
}

func (m *MapStore) UpdateNode(node *control.Node) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nodeIds[node.ID] = node
	return nil
}

func (m *MapStore) Close() error {
	return nil
}
