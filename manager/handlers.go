package manager

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/netip"
	"time"

	"github.com/caldog20/calnet/manager/store"
	"github.com/caldog20/calnet/types"
	"github.com/gorilla/websocket"
)

const (
	NewKeyExpiryTime = (24 * time.Hour) * 120
)

var (
	ErrInvalidProvisionKey = errors.New("invalid provisioning key")
	ErrPeerDisabled        = errors.New("peer is disabled")
	ErrKeyExpired          = errors.New("public key is expired")
)

var upgrader = websocket.Upgrader{}

func (s *Server) LoginHandler(w http.ResponseWriter, req *http.Request) {
	nodeKeyStr := req.Header.Get("X-Node-Key")
	if nodeKeyStr == "" {
		http.Error(w, "node key not provided", http.StatusBadRequest)
		return
	}

	nodeKey := types.PublicKey{}
	err := nodeKey.UnmarshalText([]byte(nodeKeyStr))
	if err != nil {
		http.Error(w, "invalid node key", http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(w, "error reading request body", http.StatusBadRequest)
		return
	}

	loginRequest := &types.LoginRequest{}
	err = json.Unmarshal(body, loginRequest)
	if err != nil {
		http.Error(w, "malformed request body", http.StatusBadRequest)
		return
	}

	node, err := s.store.GetNodeByPublicKey(nodeKey)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			// Node is not registered
			if loginRequest.ProvisionKey != "please" {
				http.Error(
					w,
					"error registering node: invalid provision key",
					http.StatusUnauthorized,
				)
				return
			}

			peerIp, err := s.ipam.Allocate()
			if err != nil {
				http.Error(w, "error allocating node ip address", http.StatusInternalServerError)
				return
			}

			// First prefix is route list is always the base network prefix for the network
			prefix := s.ipam.GetPrefix()
			node = &types.Node{
				PublicKey: nodeKey,
				Hostname:  loginRequest.Hostname,
				Connected: false,
				Disabled:  false,
				KeyExpiry: time.Now().Add(NewKeyExpiryTime),
				TunnelIP:  peerIp,
				Routes:    []netip.Prefix{prefix},
			}

			err = s.store.CreateNode(node)
			if err != nil {
				// Release allocated IP back into the pool
				s.ipam.Release(peerIp)
				http.Error(
					w,
					fmt.Sprintf("error saving node to database: %s", err.Error()),
					http.StatusInternalServerError,
				)
				return
			}

		} else {
			// Some other error trying to retrieve node from database
			http.Error(w, "error finding node in database", http.StatusInternalServerError)
			return
		}
	} else {
		// Node was found, check if key is expired or if metadata needs an update
		// TODO: Way to renew key
		if node.IsExpired() {
			http.Error(w, "node key is expired", http.StatusUnauthorized)
			return
		}

		if node.IsDisabled() {
			http.Error(w, "node is disabled", http.StatusUnauthorized)
			return
		}
	}

	resp := types.LoginResponse{
		NodeConfig: types.NodeConfig{
			ID:       node.ID,
			TunnelIP: node.TunnelIP,
			Routes:   node.Routes,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(resp)
	if err != nil {
		slog.Error("error encoding login response", "error", err)
		return
	}
}

func (s *Server) UpdateHandler(w http.ResponseWriter, req *http.Request) {
	nodeKeyStr := req.Header.Get("X-Node-Key")
	if nodeKeyStr == "" {
		http.Error(w, "node key not provided", http.StatusBadRequest)
		return
	}

	nodeKey := types.PublicKey{}
	err := nodeKey.UnmarshalText([]byte(nodeKeyStr))
	if err != nil {
		http.Error(w, "invalid node key", http.StatusBadRequest)
		return
	}

	node, err := s.store.GetNodeByPublicKey(nodeKey)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "error: node is not registered", http.StatusNotFound)
			return
		}
		http.Error(w, "error validating node", http.StatusInternalServerError)
		return
	}

	if node.IsExpired() {
		http.Error(w, "node key is expired", http.StatusUnauthorized)
		return
	}

	if node.IsDisabled() {
		http.Error(w, "node is disabled", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		slog.Error("error upgrading websocket connection", "error", err)
		return
	}

	c := make(chan types.NodeUpdateResponse, 3)
	done := make(chan bool)
	s.nm.Subscribe(node.ID, c)

	defer func() {
		s.nm.Unsubscribe(node.ID, c)
		conn.Close()
	}()

	// TODO: Move away from ReadJSON when moving to Nacl Encryption
	go func() {
		defer close(done)
		for {
			select {
			case <-done:
				return
			case update, ok := <-c:
				if !ok {
					slog.Info("server closed update channel", "node ID", node.ID)
					return
				}
				err = conn.WriteJSON(update)
				if err != nil {
					if websocket.IsCloseError(err, websocket.CloseGoingAway) {
						return
					}
					slog.Error("error writing update response", "node ID", node.ID, "error", err)
					return
				}
			}
		}
	}()

	for {
		select {
		case <-done:
			return
		default:
		}

		update := types.NodeUpdateRequest{}
		err := conn.ReadJSON(&update)
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseGoingAway) {
				return
			}
			slog.Error("error reading update request", "node ID", node.ID, "error", err.Error())
			return
		}

		if ok := s.nm.HandleNodeUpdateRequest(nodeKey, update); !ok {
			slog.Warn(
				"error handing node update: node could be expired or disabled",
				"node ID",
				node.ID,
			)
			return
		}
	}
}
