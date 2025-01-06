package manager

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/grpcreflect"
	"github.com/caldog20/calnet/manager/types"
	mgmtv1 "github.com/caldog20/calnet/proto/gen/management/v1"
	"github.com/caldog20/calnet/proto/gen/management/v1/managementv1connect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

var (
	ErrInvalidProvisionKey = errors.New("invalid provisioning key")
	ErrPeerDisabled        = errors.New("peer is disabled")
	ErrKeyExpired          = errors.New("public key is expired")
)

type Server struct {
	listenAddr string
	store      Store
	tokens     sync.Map
	ipam       *IPAM
	debugMode  bool
}

func NewServer(address string, store Store, ipam *IPAM, debug bool) *Server {
	return &Server{
		listenAddr: address,
		store:      store,
		ipam:       ipam,
		debugMode:  debug,
	}
}

func (s *Server) Run(ctx context.Context) error {
	mux := http.NewServeMux()

	interceptors := connect.WithInterceptors(NewLogInterceptor())
	path, handler := managementv1connect.NewManagementServiceHandler(s, interceptors)
	reflector := grpcreflect.NewStaticReflector("management.v1.ManagementService")
	mux.Handle(path, handler)
	mux.Handle(grpcreflect.NewHandlerV1Alpha(reflector))

	ln, err := net.Listen("tcp", s.listenAddr)
	if err != nil {
		log.Fatal(fmt.Sprintf("error listening on %s: %v", s.listenAddr, err))
	}

	defer ln.Close()

	server := &http.Server{
		Handler: h2c.NewHandler(mux, &http2.Server{}),
	}

	go func() {
		err := server.Serve(ln)
		if err != nil && err != http.ErrServerClosed {
			log.Fatal(fmt.Sprintf("error starting http server: %v", err))
		}
	}()

	log.Printf("http server listening on %s", s.listenAddr)

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

func (s *Server) loginPeer(loginRequest *mgmtv1.LoginRequest) error {
	peer, err := s.store.GetPeer(loginRequest.PublicKey)
	if err != nil {
		// Peer was not found, try to register
		if errors.Is(err, ErrNotFound) {
			if loginRequest.GetProvisionKey() != "" {
				return s.registerPeer(loginRequest)
			}
		} else {
			return err
		}
	}

	if peer.Disabled {
		return ErrPeerDisabled
	}

	if peer.KeyExpiry.Before(time.Now()) {
		// TODO: update peers public key and reset expiry time
		return ErrKeyExpired
	}

	// TODO: Validate peer key hasn't expired or been disabled/revoked
	// TODO: Update peer metadata in database (hostname, etc)
	return nil
}

func (s *Server) registerPeer(loginRequest *mgmtv1.LoginRequest) error {
	provKey := loginRequest.GetProvisionKey()
	if provKey != "provisionme" {
		return ErrInvalidProvisionKey
	}

	peerIp, err := s.ipam.Allocate()
	if err != nil {
		return err
	}

	peer := &types.Peer{
		MachineID: loginRequest.GetMachineId(),
		PublicKey: loginRequest.PublicKey,
		Hostname:  loginRequest.Meta.GetHostname(),
		TunnelIP:  peerIp,
		Connected: false,
		Disabled:  false,
	}

	err = s.store.CreatePeer(peer)
	if err != nil {
		return err
	}

	return nil
}

func validatePublicKey(key string) error {
	_, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return err
	}
	return nil
}
