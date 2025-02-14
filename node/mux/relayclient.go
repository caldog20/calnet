package mux

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/caldog20/calnet/types"
	"github.com/gorilla/websocket"
)

var ErrNotConnected = errors.New("relay client is not connected")

type RelayClient struct {
	mu           sync.Mutex
	conn         *websocket.Conn
	connected    bool
	dialer       *websocket.Dialer
	realDialAddr string
	closed       atomic.Bool

	DialAddr string
	NodeKey  types.PublicKey
}

func (r *RelayClient) Dial() error {
	url, err := url.Parse(r.DialAddr)
	if err != nil {
		return errors.New("error parsing dial address")
	}

	var prefix string
	if url.Scheme == "https" {
		prefix = "wss"
	} else {
		prefix = "ws"
	}

	r.realDialAddr = fmt.Sprintf("%s://%s/%s", prefix, url.Host, "relay")
	log.Printf("relay dial address: %s", r.realDialAddr)

	r.dialer = &websocket.Dialer{
		HandshakeTimeout: time.Second * 2,
		ReadBufferSize:   2048,
		WriteBufferSize:  2048,
	}

	go r.connect()
	return nil
}

func (r *RelayClient) IsConnected() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.connected
}

func (r *RelayClient) Close() {
	r.closed.Store(true)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.connected = false
	if r.conn != nil {
		_ = r.conn.Close()
	}
}

func (r *RelayClient) closeAndReconnect() {
	r.Close()
	r.connect()
}

func (r *RelayClient) connect() {
	for {
		header := http.Header{}
		header.Add("x-node-key", r.NodeKey.String())

		conn, _, err := r.dialer.Dial(r.realDialAddr, header)
		if err != nil {
			log.Printf("error connecting to relay websocket: %s", err)
			time.Sleep(time.Second * 2)
			continue
		}

		// We should now be connected
		r.mu.Lock()
		r.conn = conn
		r.connected = true
		r.mu.Unlock()
		log.Printf("relay websocket connection established")
		return
	}
}

func (r *RelayClient) Read() (data []byte, err error) {
	err = ErrNotConnected
	if r.IsConnected() {
		_, data, err = r.conn.ReadMessage()
		if err != nil {
			if r.closed.Load() {
				return
			}
			r.closeAndReconnect()
		}
	}
	return
}

func (r *RelayClient) Write(data []byte) (err error) {
	err = ErrNotConnected
	if r.IsConnected() {
		err = r.conn.WriteMessage(websocket.BinaryMessage, data)
		if err != nil {
			if r.closed.Load() {
				return
			}
			r.closeAndReconnect()
		}
	}
	return
}
