package node

import (
	"context"
	"log"
	"net"
	"sync"

	"github.com/caldog20/calnet/node/mux"
	"github.com/caldog20/calnet/types"
)

type Node struct {
	ID uint64
	// TODO: Logic to update keys
	PrivateKey types.PrivateKey
	PublicKey  types.PublicKey

	Client *Client
	mux    *mux.Mux
	// tunnel configuration

	// Peers
	mu       sync.Mutex
	peers    map[uint64]*mux.Conn
	hostname string
}

func Run(ctx context.Context, server, hostname string) error {
	node := &Node{hostname: hostname}
	priv := types.NewPrivateKey()
	// priv.UnmarshalText([]byte("oyQIxM6R7lEGEu8yiOlV2AhM786EcLTzyKfirhJRZmA="))
	node.PrivateKey = priv
	node.PublicKey = node.PrivateKey.Public()
	client := NewClient(server, node.PublicKey)

	node.Client = client
	node.mu = sync.Mutex{}
	node.peers = make(map[uint64]*mux.Conn)

	config, err := client.Login(hostname)
	if err != nil {
		log.Fatal(err)
	}

	node.ID = config.ID

	c, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		log.Fatal(err)
	}

	node.mux = mux.NewConnMux(node.ID, node.PublicKey, c, server)
	defer node.mux.Close()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		update, err := client.Poll()
		if err != nil {
			log.Println(err)
			continue
		}
		if update != nil {
			if update.Peers != nil {
				log.Printf("received update with %d remote peers", len(update.Peers))
				node.mu.Lock()
				for _, peer := range update.Peers {
					_, ok := node.peers[peer.ID]
					if !ok {
						c, err := node.mux.GetConn(peer.ID, peer.PublicKey)
						if err != nil {
							log.Println(err)
							continue
						}
						node.peers[peer.ID] = c
						if hostname == "test" {
							go writePeerConn(c)
						}
					} else {
						node.mux.RemoveConn(peer.PublicKey)
						c, err := node.mux.GetConn(peer.ID, peer.PublicKey)
						if err != nil {
							log.Println(err)
							continue
						}
						node.peers[peer.ID] = c
					}
				}
				node.mu.Unlock()
			}
		}

	}
}

func writePeerConn(c *mux.Conn) {
	s := []byte("Hello Conn")
	// for {
	// time.Sleep(time.Second * 3)
	c.Write(s)
	// }
}
