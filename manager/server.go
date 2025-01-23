package manager

import (
	"encoding/base64"
	"net/http"

	"github.com/caldog20/calnet/manager/ipam"
	"github.com/caldog20/calnet/manager/store"
)

type Server struct {
	store store.Store
	ipam  *ipam.IPAM
	nm    *NodeManager
	mux   *http.ServeMux
}

func NewServer(store store.Store, ipam *ipam.IPAM, nm *NodeManager) *Server {
	s := &Server{
		store: store,
		ipam:  ipam,
		nm:    nm,
		mux:   http.NewServeMux(),
	}

	s.configureMux()

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) configureMux() {
	// Root Handler
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not found", http.StatusNotFound)
	})

	// Login Handler
	s.mux.HandleFunc("POST /login", s.LoginHandler)

	// Websocket Updates Handler
	s.mux.HandleFunc("GET /updates", s.UpdateHandler)
}

func validatePublicKey(key string) error {
	_, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return err
	}
	return nil
}
