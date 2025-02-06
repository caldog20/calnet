package controlserver

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"log/slog"
	"net/http"
	"time"

	"github.com/caldog20/calnet/control"
	"github.com/caldog20/calnet/control/store"
	"github.com/caldog20/calnet/types"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize: 2048,
	WriteBufferSize: 2048,
}

func (s *Server) registerControlRoutes() {
	s.mux.HandleFunc("GET /key", s.handleKey)
	s.mux.HandleFunc("POST /login", s.handleLogin)
	s.mux.HandleFunc("POST /logout", s.handleLogout)
	s.mux.HandleFunc("POST /poll", s.handlePoll)
	s.mux.HandleFunc("GET /relay", s.handleRelay)
}

func (s *Server) handleKey(w http.ResponseWriter, r *http.Request) {}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	loginRequest := control.LoginRequest{}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "error reading request body", http.StatusBadRequest)
		return
	}
	err = json.Unmarshal(body, &loginRequest)
	if err != nil {
		http.Error(w, "error parsing request body", http.StatusBadRequest)
		return
	}

	node, err := s.store.GetNodeByKey(loginRequest.NodeKey)
	if err != nil && errors.Is(err, store.ErrNotFound) {
		if loginRequest.ProvisionKey != "please" {
			http.Error(w, "invalid provisioning key", http.StatusBadRequest)
			return
		}

		slog.Info(
			"creating new node",
			"node key",
			loginRequest.NodeKey,
			"hostname",
			loginRequest.Hostname,
		)
		node, err = s.createNode(loginRequest.NodeKey, loginRequest.Hostname)
		if err != nil {
			http.Error(w, "error creating node", http.StatusInternalServerError)
			return
		}
	} else if err != nil {
		http.Error(w, "error getting node by key", http.StatusInternalServerError)
		return
	}

	if node.IsExpired() {
		http.Error(w, "node is expired", http.StatusUnauthorized)
		return
	}

	resp := control.LoginResponse{
		NodeConfig: control.NodeConfig{
			ID:       node.ID,
			Prefix:   node.Prefix,
			TunnelIP: node.TunnelIP,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("login endpoint error", "error", err)
		return
	}

	s.notifyAll()
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {}

func (s *Server) handlePoll(w http.ResponseWriter, r *http.Request) {
	pollRequest := control.PollRequest{}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "error reading request body", http.StatusBadRequest)
		return
	}

	err = json.Unmarshal(body, &pollRequest)
	if err != nil {
		http.Error(w, "error parsing request body", http.StatusBadRequest)
		return
	}

	node, err := s.store.GetNodeByKey(pollRequest.NodeKey)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "node not found", http.StatusNotFound)
		} else {
			http.Error(w, "error getting node by key", http.StatusInternalServerError)
		}
		return
	}

	if node.IsExpired() {
		http.Error(w, "node is expired", http.StatusUnauthorized)
		return
	}

	notifyChan := s.getNotifyChan(node.ID)

	for {
		select {
		case <-notifyChan:
			update := s.getUpdateForNode(node.ID)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(update); err != nil {
				slog.Error("login endpoint error", "error", err)
			}
			return
		case <-r.Context().Done():
			return
		case <-time.After(time.Second * 5):
			w.WriteHeader(http.StatusRequestTimeout)
			return
		}
	}
}

func (s *Server) handleRelay(w http.ResponseWriter, r *http.Request) {
	nodeKeystr := r.Header.Get("x-node-key")
	if nodeKeystr == "" {
		http.Error(w, "invalid node key in header", http.StatusBadRequest)
		return
	}

	nodeKey := types.PublicKey{}
	err := nodeKey.UnmarshalText([]byte(nodeKeystr))
	if err != nil {
		http.Error(w, "error parsing node key", http.StatusBadRequest)
		return
	}

	node, err := s.store.GetNodeByKey(nodeKey)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "node must be registered before using relay", http.StatusBadRequest)
		} else {
			http.Error(w, "error finding node", http.StatusInternalServerError)
		}
		return
	}

	if node.IsExpired() {
		http.Error(w, "node key is expired, node needs to re-login", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("error upgrading websocket:", err)
		return
	}

	go s.relay(nodeKey, conn)
}
