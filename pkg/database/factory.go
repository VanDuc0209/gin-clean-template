package database

import (
	"fmt"
	"log"

	"github.com/duccv/go-clean-template/config"
)

type DatabaseType string

const (
	PostgreSQL   DatabaseType = "postgres"
	MongoDBNoSQL DatabaseType = "mongodb"
)

// MongoDeployment định nghĩa các kiểu deployment MongoDB
type MongoDeployment string

const (
	MongoSingle     MongoDeployment = "single"
	MongoReplicaSet MongoDeployment = "replica_set"
	MongoSharded    MongoDeployment = "sharded"
)

type Database interface {
	Connect() error
	Close() error
	Ping() error
	GetReadConnection() any
	GetWriteConnection() any
	GetType() DatabaseType
	IsConnected() bool
	HealthCheck() map[string]error
}

// DatabaseFactory là factory class để tạo database instances
type DatabaseFactory struct {
	databases map[string]Database
}

func NewDatabaseFactory() *DatabaseFactory {
	return &DatabaseFactory{
		databases: make(map[string]Database),
	}
}

// CreateDatabase tạo database instance dựa trên config
func (f *DatabaseFactory) CreateDatabase(
	name string,
	config *config.DatabaseConfig,
) (Database, error) {
	var db Database
	var err error

	switch DatabaseType(config.Type) {
	case PostgreSQL:
		db = NewPostgresDB(&config.PostgresConfig)
	case MongoDBNoSQL:
		db = NewMongoDB(&config.MongoConfig)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", config.Type)
	}

	if err = db.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", config.Type, err)
	}

	f.databases[name] = db
	return db, nil
}

// GetDatabase lấy database instance theo tên
func (f *DatabaseFactory) GetDatabase(name string) (Database, error) {
	db, exists := f.databases[name]
	if !exists {
		return nil, fmt.Errorf("database '%s' not found", name)
	}
	return db, nil
}

// CloseAll đóng tất cả database connections
func (f *DatabaseFactory) CloseAll() error {
	for name, db := range f.databases {
		if err := db.Close(); err != nil {
			log.Printf("Error closing database %s: %v", name, err)
		}
	}
	f.databases = make(map[string]Database)
	return nil
}

func (f *DatabaseFactory) Close(name string) error {
	db, exists := f.databases[name]
	if exists {
		if err := db.Close(); err != nil {
			log.Printf("Error closing database %s: %v", name, err)
		}
		delete(f.databases, name)
	}

	return nil
}

// ListDatabases trả về danh sách tất cả database instances
func (f *DatabaseFactory) ListDatabases() map[string]DatabaseType {
	result := make(map[string]DatabaseType)
	for name, db := range f.databases {
		result[name] = db.GetType()
	}
	return result
}

// HealthCheck kiểm tra tình trạng của tất cả database connections
func (f *DatabaseFactory) HealthCheck() map[string]map[string]error {
	result := make(map[string]map[string]error)
	for name, db := range f.databases {
		result[name] = db.HealthCheck()
	}
	return result
}

// RemoveDatabase xóa database instance
func (f *DatabaseFactory) RemoveDatabase(name string) error {
	db, exists := f.databases[name]
	if !exists {
		return fmt.Errorf("database '%s' not found", name)
	}

	if err := db.Close(); err != nil {
		return fmt.Errorf("failed to close database %s: %w", name, err)
	}

	delete(f.databases, name)
	return nil
}

// GetDatabaseStats lấy thống kê của database
func (f *DatabaseFactory) GetDatabaseStats() map[string]interface{} {
	stats := make(map[string]interface{})

	stats["total_databases"] = len(f.databases)

	typeCount := make(map[DatabaseType]int)
	for _, db := range f.databases {
		typeCount[db.GetType()]++
	}
	stats["by_type"] = typeCount

	// Health summary
	healthyCount := 0
	unhealthyCount := 0

	for _, db := range f.databases {
		healthCheck := db.HealthCheck()
		isHealthy := true
		for _, err := range healthCheck {
			if err != nil {
				isHealthy = false
				break
			}
		}

		if isHealthy {
			healthyCount++
		} else {
			unhealthyCount++
		}
	}

	stats["healthy"] = healthyCount
	stats["unhealthy"] = unhealthyCount

	return stats
}
