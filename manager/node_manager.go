package manager

import (
	"log"
	"sync"
	"sync/atomic"

	"github.com/caldog20/calnet/types"
)

type NodeManager struct {
	store            Store
	connectedCounter atomic.Uint64
	connected        sync.Map
	mu               sync.Mutex
	updates          map[uint64]chan types.NodeUpdateResponse
	disconnectQueue  sync.Map
}

func NewNodeManager(store Store) *NodeManager {
	return &NodeManager{
		store:   store,
		updates: make(map[uint64]chan types.NodeUpdateResponse),
	}
}

func (nm *NodeManager) Subscribe(id uint64, c chan types.NodeUpdateResponse) {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	if e, ok := nm.updates[id]; ok {
		close(e)
	}

	nm.updates[id] = c

	nm.PeerConnectedEvent(id)
	nm.queueFullUpdate(id, c)
}

func (nm *NodeManager) Unsubscribe(id uint64, c chan types.NodeUpdateResponse) {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	if e, ok := nm.updates[id]; ok {
		if e == c {
			close(c)
			delete(nm.updates, id)
			nm.PeerDisconnectedEvent(id)
		}
	}
}

func (nm *NodeManager) HandleNodeUpdateRequest(nodeKey types.PublicKey, msg types.NodeUpdateRequest) bool {
	node, err := nm.store.GetNodeByPublicKey(nodeKey)
	if err != nil {
		log.Printf("handlemessage: error getting peer by public key: %v", err)
		return false
	}
	if node.IsExpired() || node.IsDisabled() {
		// Notify other peers of expired/disabled node and remove node
		nm.revokeNodeNotify(node.ID)
		log.Printf("handlemessage: node %s is expired or is disabled, closing updates channel", nodeKey)
		return false
	}

	updated := false
	if msg.Endpoints != nil {
		log.Printf("handlemessage: updating endpoints for peer: %d", node.ID)
		node.Endpoints = msg.Endpoints
		updated = true
	}
	if msg.Hostname != "" {
		log.Printf("handlemessage: updating hostname for peer: %d", node.ID)
		node.Hostname = msg.Hostname
		updated = true
	}

	if updated {
		err := nm.store.UpdatePeer(node)
		if err != nil {
			log.Printf("handlemessage: error updating peer in db: %v", err)
			return false
		}
		nm.changedNodeNotify(node)
	}

	return true
}

func (nm *NodeManager) changedNodeNotify(node *types.Node) {
	rp := node.ToRemotePeer(true)
	update := types.NodeUpdateResponse{
		Peers: []types.RemotePeer{
			rp,
		},
	}

	nm.sendUpdateToPeers(node.ID, update)
}

func (nm *NodeManager) removeNode(id uint64) {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	if c, ok := nm.updates[id]; ok {
		close(c)
		delete(nm.updates, id)
		nm.PeerDisconnectedEvent(id)
		nm.connected.Delete(id)
	}
}

func (nm *NodeManager) queueFullUpdate(id uint64, c chan types.NodeUpdateResponse) {
	//node, err := nm.store.GetNodeByID(id)
	//if err != nil {
	//	return
	//}

	peers, err := nm.store.GetPeersOfNode(id)
	if err != nil {
		log.Printf("queuefullupdate: error getting updates: %v", err)
		return
	}

	var remotePeers []types.RemotePeer

	for _, peer := range peers {
		if peer.IsExpired() || peer.IsDisabled() || peer.Endpoints == nil {
			continue
		}
		rp := peer.ToRemotePeer(false)
		remotePeers = append(remotePeers, rp)
	}

	update := types.NodeUpdateResponse{
		//NodeConfig: &types.NodeConfig{
		//	ID:       node.ID,
		//	Routes:   node.Routes,
		//	TunnelIP: node.TunnelIP.String(),
		//},
		Peers: remotePeers,
	}

	c <- update
}

//func (nm *NodeManager) newNodeNotify(node *types.Node) {
//	update := types.NodeUpdateResponse{
//		Peers: []types.RemotePeer{node.ToRemotePeer(true)},
//	}
//
//	nm.sendUpdateToPeers(node.ID, update)
//}

func (nm *NodeManager) revokeNodeNotify(id uint64) {
	update := types.NodeUpdateResponse{
		RevokedPeers: []types.RemotePeer{
			{ID: id},
		},
	}

	nm.sendUpdateToPeers(id, update)
}

func (nm *NodeManager) sendUpdateToPeers(nodeID uint64, update types.NodeUpdateResponse) {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	for id, c := range nm.updates {
		if id != nodeID {
			select {
			case c <- update:
			default:
				log.Printf("sendUpdateToPeers: channel full update discarded")
			}
		}
	}
}

//func (nm *NodeManager) handleNodeUpdateRequest(node *types.Node, msg types.NodeUpdateRequest) {
//
//}

func (nm *NodeManager) PeerConnectedEvent(id uint64) {
	log.Printf("peer %d connected", id)
	nm.connected.Store(id, true)
}

func (nm *NodeManager) PeerDisconnectedEvent(id uint64) {
	log.Printf("peer %d disconnected", id)
	nm.connected.Store(id, false)
}
