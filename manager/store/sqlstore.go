package store

import (
	"errors"
	"net/netip"

	"github.com/caldog20/calnet/control"
	"github.com/caldog20/calnet/types"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
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
			Logger:      logger.Default.LogMode(logger.Error),
		})
	if err != nil {
		return nil, err
	}

	err = db.AutoMigrate(&control.Node{})
	if err != nil {
		return nil, err
	}

	return &SqlStore{
		db: db,
	}, nil
}

func (s *SqlStore) GetNodes() (types.Nodes, error) {
	var nodes types.Nodes
	err := s.db.Find(&nodes).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return nodes, nil
}

func (s *SqlStore) CreateNode(node *control.Node) error {
	return s.db.Create(node).Error
}

func (s *SqlStore) GetPeersOfNode(id uint64) (types.Nodes, error) {
	var nodes types.Nodes
	if err := s.db.Where("id != ?", id).Find(&nodes).Error; err != nil {
		return nil, err
	}
	return nodes, nil
}

func (s *SqlStore) GetNodeByPublicKey(publicKey types.PublicKey) (*control.Node, error) {
	var node *control.Node
	if err := s.db.Where("public_key = ?", publicKey).First(&node).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return node, nil
}

func (s *SqlStore) GetNodeByID(id uint64) (*control.Node, error) {
	var node *control.Node
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
	var nodes []control.Node
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

func (s *SqlStore) UpdateNode(node *control.Node) error {
	return s.db.Save(node).Error
}

func (s *SqlStore) Close() error {
	return nil
}
