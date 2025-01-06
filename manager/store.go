package manager

import (
	"database/sql"
	"errors"
	"net/netip"

	"github.com/caldog20/calnet/manager/types"
	_ "modernc.org/sqlite"
)

var ErrNotFound = errors.New("record not found in database")

type Store interface {
	GetPeer(publicKey string) (*types.Peer, error)
	CreatePeer(peer *types.Peer) error
	DeletePeer(publicKey string) error
	GetAllocatedIps() ([]netip.Addr, error)
	Close() error
}

type SqlStore struct {
	DB *sql.DB
}

func (s *SqlStore) GetPeer(publicKey string) (*types.Peer, error) {
	peer := &types.Peer{}
	err := s.DB.QueryRow("SELECT * FROM peers WHERE publickey = ?", publicKey).Scan(peer)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		} else {
			return nil, err
		}
	}
	return peer, nil
}

func (s *SqlStore) CreatePeer(peer *types.Peer) error {
	stmt, err := s.DB.Prepare(
		"INSERT INTO peers (publickey, tunnelip, hostname, disabled) VALUES (?, ?, ?, ?)",
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(peer.PublicKey, peer.TunnelIP.String(), peer.Hostname, peer.Connected, peer.Disabled)
	if err != nil {
		return err
	}

	return nil
}

func (s *SqlStore) DeletePeer(publicKey string) error {
	stmt, err := s.DB.Prepare("DELETE FROM peers WHERE publickey = ?")
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
	rows, err := s.DB.Query("SELECT tunnelip FROM peers")
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

	peerTable := `CREATE TABLE IF NOT EXISTS peers (
  id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
  publickey TEXT NOT NULL UNIQUE,
  hostname TEXT UNIQUE,
  tunnelip TEXT UNIQUE,
  disabled INTEGER
  );`

	_, err = db.Exec(peerTable)
	if err != nil {
		return nil, err
	}

	return &SqlStore{DB: db}, nil
}
