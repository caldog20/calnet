package store

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"net/netip"
	"os"

	"github.com/caldog20/calnet/control"
	"github.com/caldog20/calnet/types"
	bolt "go.etcd.io/bbolt"
)

var ErrNotFound = errors.New("not found in database")

type BoltStore struct {
	db *bolt.DB
}

func (b *BoltStore) GetPeersOfNode(id uint64) ([]*control.Node, error) {
	var peers []*control.Node
	err := b.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("nodes"))
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			nodeId := binary.BigEndian.Uint64(k)
			if nodeId == id {
				continue
			}
			node := &control.Node{}
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

func (b *BoltStore) GetNodeByKey(key types.PublicKey) (*control.Node, error) {
	var node *control.Node
	err := b.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("nodes"))
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			node = &control.Node{}
			err := json.Unmarshal(v, node)
			if err != nil {
				return err
			}
			if node.PublicKey == key {
				return nil
			}
		}
		return ErrNotFound
	})
	if err != nil {
		return nil, err
	}
	return node, nil
}

func (b *BoltStore) GetNodeByID(id uint64) (*control.Node, error) {
	var node *control.Node
	err := b.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("nodes"))
		v := b.Get(binary.BigEndian.AppendUint64(make([]byte, 8), id))
		if v == nil {
			return ErrNotFound
		}
		node = &control.Node{}
		err := json.Unmarshal(v, node)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return node, nil
}

func (b *BoltStore) CreateNode(node *control.Node) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("nodes"))

		id, _ := b.NextSequence()
		node.ID = id

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
	// TODO implement me
	panic("implement me")
}

func (b *BoltStore) UpdateNode(node *control.Node) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("nodes"))
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
			node := &control.Node{}
			err := json.Unmarshal(v, node)
			if err != nil {
				return err
			}
			allocatedNodeIPs = append(allocatedNodeIPs, node.TunnelIP)
			return nil
		})
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return allocatedNodeIPs, nil
}

func NewBoltStore(path string) (*BoltStore, error) {
	if _, err := os.Stat(path); err == nil {
		// Delete the file
		os.Remove(path)
	}

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
