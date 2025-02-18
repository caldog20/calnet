package controlservice

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/caldog20/calnet/control/server/config"
	"github.com/caldog20/calnet/control/server/internal/ipam"
	"github.com/caldog20/calnet/control/server/internal/node"
	"github.com/caldog20/calnet/control/server/internal/store"
	"github.com/caldog20/calnet/pkg/controlapi"
	"github.com/caldog20/calnet/pkg/keys"
)

const (
	// TODO: Make configurable
	CleanupRoutineTicker = time.Minute * 5
)

type Control struct {
	store              store.Store
	privateKey         keys.PrivateKey
	publicKey          keys.PublicKey
	ipam               *ipam.IPAM
	disableControlNacl bool

	mu           sync.Mutex
	pollingNodes map[uint64]*pollingNode
	closed       chan bool
}

type pollingNode struct {
	lastPoll time.Time
	ch       chan struct{}
}

func New(conf config.Config, store store.Store) *Control {
	privKey, err := readKeyFromFile(config.ConfigPath())
	if err != nil {
		privKey = generateKey(config.ConfigPath())
	}

	allocatedIps, err := store.GetAllocatedNodeIPs()
	if err != nil {
		log.Println("error getting allocated node IPs from store:", err)
	}

	ipam := ipam.NewIPAM(conf.NetworkPrefix, allocatedIps)

	return &Control{
		store:              store,
		ipam:               ipam,
		pollingNodes:       make(map[uint64]*pollingNode),
		closed:             make(chan bool),
		disableControlNacl: conf.Debug,
		privateKey:         privKey,
		publicKey:          privKey.PublicKey(),
	}
}

func (c *Control) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /key", c.handleControlKey)
	mux.HandleFunc("POST /login", c.handleLogin)
	mux.HandleFunc("POST /poll", c.handlePoll)
}

func (c *Control) VerifyKeyForRelay(key keys.PublicKey) bool {
	n, err := c.store.GetNodeByKey(key)
	if err != nil {
		return false
	}
	if n.IsExpired() {
		return false
	}
	if n.IsDisabled() {
		return false
	}
	return true
}

func (c *Control) cleanupPollingNodes() {
	t := time.NewTicker(CleanupRoutineTicker)
	for {
		select {
		case <-t.C:
			c.mu.Lock()
			for id, nn := range c.pollingNodes {
				if nn.lastPoll.Minute() > 5 {
					close(nn.ch)
					delete(c.pollingNodes, id)
				}
			}
			c.mu.Unlock()
		case <-c.closed:
			return
		}
	}
}

func (c *Control) getNodePollChan(id uint64) chan struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	pn, ok := c.pollingNodes[id]
	if !ok {
		pn = &pollingNode{
			ch: make(chan struct{}, 1),
		}
		c.pollingNodes[id] = pn
		pn.ch <- struct{}{}
	}

	pn.lastPoll = time.Now()

	return pn.ch
}

func (c *Control) closeAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Closed() {
		return
	}
	for _, nn := range c.pollingNodes {
		close(nn.ch)
	}
}

func (c *Control) createNode(nodeKey keys.PublicKey) (*node.Node, error) {
	nodeIP, err := c.ipam.Allocate()
	if err != nil {
		return nil, err
	}

	n := &node.Node{
		NodeKey:   nodeKey,
		KeyExpiry: time.Now().Add(node.DefaultKeyExpiryDuration),
		IP:        nodeIP,
		Prefix:    c.ipam.GetPrefix(),
	}

	err = c.store.CreateNode(n)
	if err != nil {
		c.ipam.Release(nodeIP)
		return nil, err
	}

	return n, nil
}

func (c *Control) getUpdate(n *node.Node) (*controlapi.PollResponse, error) {
	peers, err := c.store.GetPeersOfNode(n.ID)
	if err != nil {
		return nil, err
	}

	config := c.getNodeConfig(n)

	resp := &controlapi.PollResponse{
		Peers:  make([]controlapi.Peer, len(peers)),
		Config: config,
	}

	for _, p := range peers {
		resp.Peers = append(resp.Peers, controlapi.Peer{
			ID:        p.ID,
			PublicKey: p.NodeKey,
			IP:        p.IP,
		})
	}
	return resp, nil
}

func (c *Control) getNodeConfig(n *node.Node) *controlapi.NodeConfig {
	return &controlapi.NodeConfig{
		ID:     n.ID,
		IP:     n.IP,
		Prefix: n.Prefix,
	}
}

func (c *Control) notifyOne(id uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if pn, ok := c.pollingNodes[id]; ok {
		select {
		case pn.ch <- struct{}{}:
		default:
		}
	}
}

func (c *Control) notifyAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, nn := range c.pollingNodes {
		select {
		case nn.ch <- struct{}{}:
		default:
		}
	}
}

func (c *Control) Close() {
	if !c.Closed() {
		close(c.closed)
	}
}

func (c *Control) Closed() bool {
	select {
	case <-c.closed:
		return true
	default:
		return false
	}
}

func readKeyFromFile(path string) (keys.PrivateKey, error) {
	keyPath := filepath.Join(path, "private_key")
	f, err := os.Open(keyPath)
	if err != nil {
		return keys.PrivateKey{}, err
	}
	defer f.Close()

	jsonKey := struct {
		PrivateKey keys.PrivateKey
	}{}

	err = json.NewDecoder(f).Decode(&jsonKey)
	if err != nil {
		return keys.PrivateKey{}, err
	}
	if jsonKey.PrivateKey.IsZero() {
		return keys.PrivateKey{}, errors.New("private key is invalid")
	}
	return jsonKey.PrivateKey, nil
}

func generateKey(path string) keys.PrivateKey {
	p := keys.NewPrivateKey()
	keyPath := filepath.Join(path, "private_key")
	f, err := os.OpenFile(keyPath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		log.Printf(
			"error opening file to write private key to disk - key will be ephemeral - %s",
			err,
		)
		return p
	}
	defer f.Close()

	jsonKey := struct {
		PrivateKey keys.PrivateKey
	}{
		PrivateKey: p,
	}

	if err = json.NewEncoder(f).Encode(jsonKey); err != nil {
		log.Printf("error encoding json for private key storage: %s", err)
	}
	return p
}
