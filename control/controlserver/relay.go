package controlserver

import (
	"log"

	"github.com/caldog20/calnet/types"
	"github.com/gorilla/websocket"
)

func (s *Server) registerRelayConn(nodeKey types.PublicKey, conn *websocket.Conn) {
	log.Println("registering websocket conn for key:", nodeKey.String())
	
	s.relayMu.Lock()
	defer s.relayMu.Unlock()

	ec, ok := s.relayCons[nodeKey]
	if ok && ec != nil {
		log.Printf("closing existing websocket connection for node key %s", nodeKey.String())
		ec.Close()
	}

	s.relayCons[nodeKey] = conn
}

func (s *Server) removeRelayConn(nodeKey types.PublicKey, conn *websocket.Conn) {
	log.Println("removing websocket conn for key:", nodeKey.String())
	
	s.relayMu.Lock()
	defer s.relayMu.Unlock()

	c, ok := s.relayCons[nodeKey]
	if !ok {
		return
	}

	if c != conn {
		return
	}

	delete(s.relayCons, nodeKey)
}

func (s *Server) relay(nodeKey types.PublicKey, conn *websocket.Conn) {
	s.registerRelayConn(nodeKey, conn)
	defer conn.Close()
	defer s.removeRelayConn(nodeKey, conn)
	s.relayLoop(nodeKey, conn)
}

func (s *Server) relayLoop(nodeKey types.PublicKey, conn *websocket.Conn) {
	serverClosing := func() bool {
		select {
		case <-s.closed:
			return true
		default:
			return false
		}
	}

	for {
		if serverClosing() {
			return
		}

		_, packet, err := conn.ReadMessage()
		if err != nil {
			log.Println("websocket error:", err)
			return
		}

		if len(packet) < 32 {
			log.Println("relay packet too short")
			continue
		}

		dst := packet[:32]
		dstKey := types.PublicKeyFromRawBytes(dst)
		
		// Lock since we can only have one concurrent writer to any websocket conn
		s.relayMu.Lock()
		
		dstConn, ok := s.relayCons[dstKey]
		if !ok {
			log.Println("relay conn not found for destination")
			s.relayMu.Unlock()
			continue
		}
		
		data := nodeKey.Raw()
		data = append(data, packet[32:]...)
		err = dstConn.WriteMessage(websocket.BinaryMessage, data)
		
		s.relayMu.Unlock()
		
		if err != nil {
			log.Println("error writing to destination relay conn:", err)
			continue
		}
	}
}
