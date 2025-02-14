package controlserver

import (
	"context"
	"log"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"sync"
	"time"

	"github.com/caldog20/calnet/control"
	"github.com/caldog20/calnet/types"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/acme/autocert"
)

const (
  HTTPShutdownCtxTimeout = time.Second * 5
)

type Store interface {
	GetPeersOfNode(id uint64) ([]*control.Node, error)
	GetNodeByKey(key types.PublicKey) (*control.Node, error)
	GetNodeByID(id uint64) (*control.Node, error)
	CreateNode(node *control.Node) error
	DeleteNode(id uint64) error
	UpdateNode(node *control.Node) error
	GetAllocatedNodeIPs() ([]netip.Addr, error)
}

type Server struct {
	ln    net.Listener
	srv   *http.Server
	mux   *http.ServeMux
	store Store
	ipam  *IPAM

	ListenAddr     string
	AutocertDomain string

	mu             sync.Mutex
	notifyChannels map[uint64]chan struct{}
    peerLastSeenByID map[uint64]time.Time

	relayMu   sync.Mutex
	relayCons map[types.PublicKey]*websocket.Conn

	closed chan struct{}
}

func NewServer(store Store) *Server {
	allocatedIps, err := store.GetAllocatedNodeIPs()
	if err != nil {
		slog.Error("error getting allocated node ips", "error", err)
	}

	s := &Server{
		store:          store,
		ipam:           NewIPAM(netip.MustParsePrefix("100.70.0.0/24"), allocatedIps),
		mux:            http.NewServeMux(),
		mu:             sync.Mutex{},
		notifyChannels: make(map[uint64]chan struct{}),
		relayCons:      make(map[types.PublicKey]*websocket.Conn),
        peerLastSeenByID: make(map[uint64]time.Time),
		closed:         make(chan struct{}),
	}

	s.srv = &http.Server{
		Handler: s.mux,
	}

	s.registerControlRoutes()
	// s.registerHealthRoutes()

	return s
}

func (s *Server) ListenAndServe() error {
	var err error
	if s.AutocertDomain != "" {
		s.ln = autocert.NewListener(s.AutocertDomain)
	} else {
		if s.ln, err = net.Listen("tcp", s.ListenAddr); err != nil {
			return err
		}
	}

	log.Printf("control server listening on %s", s.ln.Addr().String())
	go s.srv.Serve(s.ln)
	return nil
}

func (s *Server) Close() error {
	close(s.closed)
	ctx, cancel := context.WithTimeout(context.Background(), HTTPShutdownCtxTimeout)
	defer cancel()
	return s.srv.Shutdown(ctx)
}
