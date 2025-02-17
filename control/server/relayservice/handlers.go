package relayservice

import (
	"log"
	"net/http"

	"github.com/caldog20/calnet/pkg/keys"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  2048,
	WriteBufferSize: 2048,
}

func (r *Relay) handleRelay(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "GET", "POST":
	default:
		http.Error(w, "invalid http method, must use GET or POST", http.StatusBadRequest)
		return
	}

	nodeKeyStr := req.Header.Get("x-node-key")
	nodeKey := keys.PublicKey{}
	err := nodeKey.DecodeFromString(nodeKeyStr)
	if err != nil {
		http.Error(w, "error node control key", http.StatusBadRequest)
		return
	}

	if verified := r.verifyKey(nodeKey); !verified {
		http.Error(w, "error validating node key", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		log.Println("error upgrading websocket conn:", err)
		return
	}

	go r.handleRelayConn(nodeKey, conn)
}
