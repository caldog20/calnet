package controlserver

import (
	"log/slog"
	"time"

	"github.com/caldog20/calnet/control"
	"github.com/caldog20/calnet/types"
)

func (s *Server) getNotifyChan(id uint64) chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch, ok := s.notifyChannels[id]
	if !ok {
		ch = make(chan struct{}, 1)
		s.notifyChannels[id] = ch
		ch <- struct{}{}
	}
	return ch
}

func (s *Server) getUpdateForNode(id uint64) *control.PollResponse {
	peers, err := s.store.GetPeersOfNode(id)
	if err != nil {
		slog.Error("update: error getting peers", "node", id, "error", err)
		return nil
	}

	var remotePeers []control.RemotePeer
	for _, peer := range peers {
		remotePeers = append(remotePeers, peer.ToRemotePeer())
	}

	return &control.PollResponse{
		Peers: remotePeers,
	}
}

func (s *Server) notifyAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ch := range s.notifyChannels {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func (s *Server) createNode(key types.PublicKey, hostname string) (*control.Node, error) {
	ip, err := s.ipam.Allocate()
	if err != nil {
		return nil, err
	}

	node := &control.Node{
		Hostname:  hostname,
		PublicKey: key,
		KeyExpiry: time.Now().Add((time.Hour * 24) * 120),
		TunnelIP:  ip,
		Prefix:    s.ipam.GetPrefix(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = s.store.CreateNode(node)
	if err != nil {
		s.ipam.Release(ip)
		return nil, err
	}

	return node, nil
}
