package store

import (
	"errors"
	"net/netip"

	"github.com/caldog20/calnet/manager"
	"github.com/caldog20/calnet/types"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	_ "modernc.org/sqlite"
)

var ErrNotFound = errors.New("not found in database")

type SqlStore struct {
	db *gorm.DB
}

func NewSqlStore(path string) (*SqlStore, error) {
	if path == "" {
		return nil, errors.New("sqlite db file path required")
	}

	db, err := gorm.Open(
		sqlite.Open(path+"?cache=shared&_journal_mode=WAL&_synchronous=1"),
		&gorm.Config{
			PrepareStmt: true,
			Logger:      logger.Default.LogMode(logger.Silent),
		})
	if err != nil {
		return nil, err
	}

	err = db.AutoMigrate(&manager.Node{})
	if err != nil {
		return nil, err
	}

	return &SqlStore{
		db: db,
	}, nil
}

func (s *SqlStore) GetNodes() (manager.Nodes, error) {
	var nodes manager.Nodes
	err := s.db.Find(&nodes).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return nodes, nil
}

func (s *SqlStore) CreateNode(node *manager.Node) error {
	return s.db.Create(node).Error
}

func (s *SqlStore) GetPeersOfNode(id uint64) (manager.Nodes, error) {
	var nodes manager.Nodes
	if err := s.db.Where("id != ?", id).Find(&nodes).Error; err != nil {
		return nil, err
	}
	return nodes, nil
}

func (s *SqlStore) GetNodeByPublicKey(publicKey types.PublicKey) (*manager.Node, error) {
	var node *manager.Node
	if err := s.db.Where("public_key = ?", publicKey).First(&node).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return node, nil
}

func (s *SqlStore) GetNodeByID(id uint64) (*manager.Node, error) {
	var node *manager.Node
	if err := s.db.First(&node, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return node, nil
}

func (s *SqlStore) DeleteNode(publicKey types.PublicKey) error {
	//TODO implement me
	panic("implement me")
}

func (s *SqlStore) GetAllocatedIps() ([]netip.Addr, error) {
	var nodes []manager.Node
	if err := s.db.Find(&nodes).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	var ips []netip.Addr
	for _, node := range nodes {
		ips = append(ips, node.TunnelIP)
	}

	return ips, nil
}

func (s *SqlStore) UpdateNode(node *manager.Node) error {
	return s.db.Save(node).Error
}

func (s *SqlStore) Close() error {
	return nil
}
