package relayservice

import (
	"errors"
	"log"
	"net/http"
	"sync"

	"github.com/caldog20/calnet/pkg/keys"
	"github.com/gorilla/websocket"
)

type Relay struct {
	closed    chan bool
	verifyKey func(keys.PublicKey) bool
	mu        sync.Mutex
	conns     map[keys.PublicKey]*websocket.Conn
}

func New() *Relay {
	return &Relay{
		closed: make(chan bool),
		conns:  make(map[keys.PublicKey]*websocket.Conn),
	}
}

func (r *Relay) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/relay", r.handleRelay)
}

func (r *Relay) SetKeyVerifier(f func(keys.PublicKey) bool) {
	r.verifyKey = f
}

func (r *Relay) registerRelayConn(node keys.PublicKey, conn *websocket.Conn) {
	log.Println("registering websocket conn for key:", node.EncodeToString())
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.conns[node]
	if ok {
		log.Println("closing existing websocket conn for key:", node.EncodeToString())
		existing.Close()
	}

	r.conns[node] = conn
}

func (r *Relay) deregisterRelayConn(node keys.PublicKey) {
	log.Println("de-registering websocket conn for key:", node.EncodeToString())

	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.conns[node]
	if ok {
		c.Close()
		delete(r.conns, node)
	}
}

func (r *Relay) handleRelayConn(node keys.PublicKey, conn *websocket.Conn) {
	r.registerRelayConn(node, conn)
	defer r.deregisterRelayConn(node)

	for {
		if r.Closed() {
			return
		}

		_, packet, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(
				err,
				websocket.CloseNormalClosure,
				websocket.CloseGoingAway,
				websocket.CloseAbnormalClosure,
			) {
				log.Printf(
					"error reading from relay conn for key: %s - %s",
					node.EncodeToString(),
					err,
				)
			}
			return
		}
		if err = r.relayPacket(packet, node); err != nil {
			log.Printf("error relaying packet: %s", err)
		}
	}
}

// TODO: Add metrics for relayed packets and failures
func (r *Relay) relayPacket(data []byte, src keys.PublicKey) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(data) < keys.PublicKeyLen {
		return errors.New("invalid packet length")
	}

	dstKey := keys.NewPublicKeyFromRawBytes(data[:keys.PublicKeyLen])

	dstConn, ok := r.conns[dstKey]
	if !ok {
		return errors.New("invalid destination key")
	}

	header := src.Raw()
	packet := append(header, data[keys.PublicKeyLen:]...)
	err := dstConn.WriteMessage(websocket.BinaryMessage, packet)
	if err != nil {
		return err
	}

	return nil
}

func (r *Relay) Close() {
	if !r.Closed() {
		close(r.closed)
	}
}

func (r *Relay) Closed() bool {
	select {
	case <-r.closed:
		return true
	default:
		return false
	}
}
