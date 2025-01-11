package manager

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"
)

var (
	ErrInvalidProvisionKey = errors.New("invalid provisioning key")
	ErrPeerDisabled        = errors.New("peer is disabled")
	ErrKeyExpired          = errors.New("public key is expired")
)

type Server struct {
	listenAddr string
	store      Store
	ipam       *IPAM
	debugMode  bool
	nm         *NodeManager
}

func NewServer(address string, store Store, ipam *IPAM, debug bool) *Server {
	return &Server{
		listenAddr: address,
		store:      store,
		ipam:       ipam,
		debugMode:  debug,
		nm:         NewNodeManager(store),
	}
}

func (s *Server) Run(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /login", s.LoginHandler)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not found", http.StatusNotFound)
	})
	mux.HandleFunc("GET /updates", s.UpdateHandler)

	ln, err := net.Listen("tcp", s.listenAddr)
	if err != nil {
		log.Fatal(fmt.Sprintf("error listening on %s: %v", s.listenAddr, err))
	}

	defer ln.Close()

	server := &http.Server{
		Handler: mux,
	}

	go func() {
		err := server.Serve(ln)
		if err != nil && err != http.ErrServerClosed {
			log.Fatal(fmt.Sprintf("error starting http server: %v", err))
		}
	}()

	log.Printf("http server listening on %s", ln.Addr().String())

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	err = server.Shutdown(shutdownCtx)
	if err != nil {
		return err
	}

	log.Println("http server shutdown complete")
	return nil
}

func validatePublicKey(key string) error {
	_, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return err
	}
	return nil
}
