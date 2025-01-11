package manager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/caldog20/calnet/types"
	"github.com/coder/websocket"
)

const (
	NewKeyExpiryTime = (24 * time.Hour) * 30
)

func (s *Server) LoginHandler(w http.ResponseWriter, req *http.Request) {
	nodeKey := req.Header.Get("X-Node-Key")
	if nodeKey == "" {
		http.Error(w, "node key not provided", http.StatusBadRequest)
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
		if errors.Is(err, ErrNotFound) {
			// Node is not registered
			if loginRequest.ProvisionKey != "please" {
				http.Error(w, "error registering node: invalid provision key", http.StatusUnauthorized)
				return
			}

			peerIp, err := s.ipam.Allocate()
			if err != nil {
				http.Error(w, "error allocating node ip address", http.StatusInternalServerError)
				return
			}

			// First prefix is route list is always the base network prefix for the network
			prefix := s.ipam.GetPrefix()
			node := &types.Node{
				PublicKey: nodeKey,
				Hostname:  loginRequest.Hostname,
				Connected: false,
				Disabled:  false,
				KeyExpiry: time.Now().Add(NewKeyExpiryTime),
				TunnelIP:  peerIp,
				Routes: []types.Route{
					{prefix},
				},
			}

			err = s.store.CreateNode(node)
			if err != nil {
				// Release allocated IP back into the pool
				s.ipam.Release(peerIp)
				http.Error(w, fmt.Sprintf("error saving node to database: %s", err.Error()), http.StatusInternalServerError)
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

	//w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

func (s *Server) UpdateHandler(w http.ResponseWriter, req *http.Request) {
	nodeKey := req.Header.Get("X-Node-Key")
	if nodeKey == "" {
		http.Error(w, "node key not provided", http.StatusBadRequest)
		return
	}

	node, err := s.store.GetNodeByPublicKey(nodeKey)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
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

	conn, err := websocket.Accept(w, req, nil)
	if err != nil {
		slog.Error("error upgrading websocket connection", "error", err)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	c := make(chan types.NodeUpdateResponse, 3)
	s.nm.Subscribe(node.ID, c)

	defer func() {
		cancel()
		conn.CloseNow()
		s.nm.Unsubscribe(node.ID, c)
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case update, ok := <-c:
				if !ok {
					slog.Info("server closed update channel", "node ID", node.ID)
					return
				}
				data, err := json.Marshal(update)
				if err != nil {
					slog.Error("error marshaling update response", "node ID", node.ID, "error", err)
					return
				}
				err = conn.Write(ctx, websocket.MessageText, data)
				if err != nil {
					slog.Error("error writing update response", "node ID", node.ID, "error", err)
					return
				}
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			_, data, err := conn.Read(ctx)
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					return
				}
				if errors.As(err, &websocket.CloseError{}) {
					return
				}
				slog.Error("error reading update request", "node ID", node.ID, "error", err.Error())
				return
			}

			update := types.NodeUpdateRequest{}
			err = json.Unmarshal(data, &update)
			if err != nil {
				slog.Error("error unmarshalling update request", "node ID", node.ID, "error", err.Error())
				return
			}
			if ok := s.nm.HandleNodeUpdateRequest(nodeKey, update); !ok {
				return
			}
		}
	}
}
