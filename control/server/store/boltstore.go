package store

import (
	"encoding/binary"
	"encoding/json"
	"net/netip"
	"time"

	"github.com/caldog20/calnet/control/server/internal/node"
	"github.com/caldog20/calnet/pkg/keys"
	bolt "go.etcd.io/bbolt"
)

type BoltStore struct {
	db *bolt.DB
}

func (b *BoltStore) GetNodes() ([]node.Node, error) {
	var nodes []node.Node
	if err := b.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("nodes"))
		err := b.ForEach(func(k, v []byte) error {
			n := node.Node{}
			err := json.Unmarshal(v, &n)
			if err != nil {
				return err
			}
			nodes = append(nodes, n)
			return nil
		})
		return err
	}); err != nil {
		return nil, err
	}

	return nodes, nil
}

func (b *BoltStore) GetPeersOfNode(id uint64) ([]*node.Node, error) {
	var peers []*node.Node
	_, err := b.GetNodeByID(id)
	if err != nil {
		return nil, err
	}

	err = b.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("nodes"))
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			nodeId := binary.BigEndian.Uint64(k)
			if nodeId == id {
				continue
			}
			node := &node.Node{}
			err := json.Unmarshal(v, node)
			if err != nil {
				return err
			}
			peers = append(peers, node)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return peers, nil
}

func (b *BoltStore) GetNodeByKey(key keys.PublicKey) (*node.Node, error) {
	var n *node.Node
	err := b.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("nodes"))
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			n = &node.Node{}
			err := json.Unmarshal(v, n)
			if err != nil {
				return err
			}
			if n.NodeKey == key {
				return nil
			}
		}
		return ErrNodeNotFound
	})
	if err != nil {
		return nil, err
	}
	return n, nil
}

func (b *BoltStore) GetNodeByID(id uint64) (*node.Node, error) {
	var n *node.Node
	err := b.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("nodes"))
		v := b.Get(itob(id))
		if v == nil {
			return ErrNodeNotFound
		}
		n = &node.Node{}
		err := json.Unmarshal(v, n)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return n, nil
}

func (b *BoltStore) CreateNode(node *node.Node) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("nodes"))

		id, _ := b.NextSequence()
		node.ID = id
		node.CreatedAt = time.Now()
		data, err := json.Marshal(node)
		if err != nil {
			return err
		}

		return b.Put(itob(id), data)
	})
}

// itob returns an 8-byte big endian representation of v.
func itob(v uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, v)
	return b
}

func (b *BoltStore) DeleteNode(id uint64) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("nodes"))
		b.Delete(itob(id))
		return nil
	})
}

func (b *BoltStore) UpdateNode(node *node.Node) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("nodes"))
		node.UpdatedAt = time.Now()
		data, err := json.Marshal(node)
		if err != nil {
			return err
		}
		return b.Put(itob(node.ID), data)
	})
}

func (b *BoltStore) GetAllocatedNodeIPs() ([]netip.Addr, error) {
	var allocatedNodeIPs []netip.Addr
	err := b.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("nodes"))
		err := b.ForEach(func(k, v []byte) error {
			n := &node.Node{}
			err := json.Unmarshal(v, n)
			if err != nil {
				return err
			}
			allocatedNodeIPs = append(allocatedNodeIPs, n.IP)
			return nil
		})
		return err
	})
	if err != nil {
		return nil, err
	}
	return allocatedNodeIPs, nil
}

func NewBoltStore(path string) (*BoltStore, error) {
	// // TODO: Currently for debugging testing
	// if _, err := os.Stat(path); err == nil {
	// 	// Delete the file
	// 	os.Remove(path)
	// }

	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, err
	}

	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("nodes"))
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &BoltStore{db: db}, nil
}

func (b *BoltStore) Close() error {
	return b.db.Close()
}
