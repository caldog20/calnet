package manager

import (
	"database/sql"
	"errors"
	"net/netip"
	"time"

	"github.com/caldog20/calnet/types"
	_ "modernc.org/sqlite"
)

var ErrNotFound = errors.New("record not found in database")

type Store interface {
	GetNodes() (types.Nodes, error)
	GetPeersOfNode(id uint64) (types.Nodes, error)
	GetNodesByPublicKey(publicKey string) (*types.Node, error)
	GetNodeByID(id uint64) (*types.Node, error)
	CreateNode(peer *types.Node) error
	DeleteNode(publicKey string) error
	GetAllocatedIps() ([]netip.Addr, error)
	UpdatePeer(peer *types.Node) error
	Close() error
}

type SqlStore struct {
	DB *sql.DB
}

func parseNode(row *sql.Rows) (*types.Node, error) {
	var expireTime int64
	var ipString string

	node := &types.Node{}
	err := row.Scan(&node.ID, &node.PublicKey, &expireTime, &node.Hostname, &ipString, &node.Disabled, &node.Endpoints, &node.Routes)
	if err != nil {
		return nil, err
	}
	node.KeyExpiry = time.Unix(0, expireTime)
	node.TunnelIP = netip.MustParseAddr(ipString)

	return node, nil
}

func (s *SqlStore) GetPeersOfNode(id uint64) (types.Nodes, error) {
	stmt, err := s.DB.Prepare("SELECT * FROM nodes WHERE id != ? ORDER BY id ASC")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, err := stmt.Query(id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes types.Nodes

	for rows.Next() {
		node, err := parseNode(rows)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

func (s *SqlStore) GetNodes() (types.Nodes, error) {
	stmt, err := s.DB.Prepare("SELECT * FROM nodes ORDER BY id ASC")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, err := stmt.Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes types.Nodes

	for rows.Next() {
		node, err := parseNode(rows)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

func (s *SqlStore) UpdatePeer(peer *types.Node) error {
	stmt, err := s.DB.Prepare("UPDATE nodes SET public_key = ?, key_expiry = ?, tunnel_ip = ?, hostname = ?, disabled = ?, endpoints = ?, routes = ? WHERE id = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(peer.PublicKey, peer.KeyExpiry.UnixNano(), peer.TunnelIP.String(), peer.Hostname, peer.Disabled, peer.Endpoints, peer.Routes, peer.ID)
	if err != nil {
		return err
	}

	return nil
}

func (s *SqlStore) GetNodeByID(id uint64) (*types.Node, error) {
	peer := &types.Node{}
	var expireTime int64
	var ipString string
	err := s.DB.QueryRow("SELECT * FROM nodes WHERE id = ?", id).Scan(&peer.ID, &peer.PublicKey, &expireTime, &peer.Hostname, &ipString, &peer.Disabled, &peer.Endpoints, &peer.Routes)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		} else {
			return nil, err
		}
	}
	peer.KeyExpiry = time.Unix(0, expireTime)
	peer.TunnelIP = netip.MustParseAddr(ipString)
	return peer, nil
}

func (s *SqlStore) GetNodesByPublicKey(publicKey string) (*types.Node, error) {
	peer := &types.Node{}
	var expireTime int64
	var ipString string
	err := s.DB.QueryRow("SELECT * FROM nodes WHERE public_key = ?", publicKey).Scan(&peer.ID, &peer.PublicKey, &expireTime, &peer.Hostname, &ipString, &peer.Disabled, &peer.Endpoints, &peer.Routes)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		} else {
			return nil, err
		}
	}
	peer.KeyExpiry = time.Unix(0, expireTime)
	peer.TunnelIP = netip.MustParseAddr(ipString)
	return peer, nil
}

func (s *SqlStore) CreateNode(peer *types.Node) error {
	stmt, err := s.DB.Prepare(
		"INSERT INTO nodes (public_key, key_expiry, tunnel_ip, hostname, disabled, endpoints, routes) VALUES (?, ?, ?, ?, ?, ?, ?)",
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	res, err := stmt.Exec(peer.PublicKey, peer.KeyExpiry.UnixNano(), peer.TunnelIP.String(), peer.Hostname, peer.Disabled, peer.Endpoints, peer.Routes)
	if err != nil {
		return err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return err
	}

	peer.ID = uint64(id)

	return nil
}

func (s *SqlStore) DeleteNode(publicKey string) error {
	stmt, err := s.DB.Prepare("DELETE FROM nodes WHERE public_key = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(publicKey)
	if err != nil {
		return err
	}

	return nil
}

func (s *SqlStore) GetAllocatedIps() ([]netip.Addr, error) {
	var allocated []netip.Addr
	rows, err := s.DB.Query("SELECT tunnel_ip FROM nodes")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var ipString string
		err = rows.Scan(&ipString)
		if err != nil {
			return nil, err
		}
		ip, err := netip.ParseAddr(ipString)
		if err != nil {
			return nil, err
		}
		allocated = append(allocated, ip)
	}

	return allocated, nil
}

func (s *SqlStore) Close() error {
	return s.DB.Close()
}

func NewSqlStore(path string) (Store, error) {
	db, err := sql.Open("sqlite", path+"?cache=shared&_journal_mode=WAL&_synchronous=1")
	if err != nil {
		return nil, err
	}

	peerTable := `CREATE TABLE IF NOT EXISTS nodes (
  id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
  public_key TEXT NOT NULL UNIQUE,
  key_expiry INTEGER,
  hostname TEXT UNIQUE,
  tunnel_ip TEXT UNIQUE,
  disabled INTEGER,
  endpoints TEXT,
  routes TEXT
  );`

	_, err = db.Exec(peerTable)
	if err != nil {
		return nil, err
	}

	return &SqlStore{DB: db}, nil
}
