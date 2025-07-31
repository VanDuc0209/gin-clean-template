package database

import (
	"context"
	"fmt"

	"github.com/duccv/go-clean-template/config"
	"github.com/duccv/go-clean-template/pkg/metrics"
	"go.uber.org/zap"
)

// DatabaseType represents the type of database
type DatabaseType string

const (
	DatabaseTypePostgreSQL DatabaseType = "postgresql"
	DatabaseTypeMongoDB    DatabaseType = "mongodb"
)

// DatabaseManager provides a unified interface for database operations
type DatabaseManager interface {
	// HealthCheck performs a comprehensive health check
	HealthCheck(ctx context.Context) error

	// Ping checks if the database is accessible
	Ping(ctx context.Context) error

	// Close closes the database connection
	Close() error

	// GetConnectionStats returns current connection pool statistics
	GetConnectionStats() (active, idle, max int)
}

// DatabaseFactory manages multiple database connections
type DatabaseFactory struct {
	postgres *PostgreSQLManager
	mongo    *MongoManager
	logger   *zap.Logger
	metrics  *metrics.MetricsCollector
}

// NewDatabaseFactory creates a new database factory
func NewDatabaseFactory(
	env *config.Env,
	logger *zap.Logger,
	metrics *metrics.MetricsCollector,
) (*DatabaseFactory, error) {
	factory := &DatabaseFactory{
		logger:  logger,
		metrics: metrics,
	}

	// Initialize PostgreSQL if configured
	if env.PostgresConfig.Host != "" {
		postgres, err := NewPostgreSQLManager(env.PostgresConfig, logger, metrics)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize PostgreSQL: %w", err)
		}
		factory.postgres = postgres
		logger.Info("PostgreSQL manager initialized")
	}

	// Initialize MongoDB if configured
	if env.MongoConfig.URI != "" {
		mongo, err := NewMongoManager(env.MongoConfig, logger, metrics)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize MongoDB: %w", err)
		}
		factory.mongo = mongo
		logger.Info("MongoDB manager initialized")
	}

	if factory.postgres == nil && factory.mongo == nil {
		return nil, fmt.Errorf("no database configured")
	}

	return factory, nil
}

// GetPostgreSQL returns the PostgreSQL manager
func (df *DatabaseFactory) GetPostgreSQL() *PostgreSQLManager {
	return df.postgres
}

// GetMongoDB returns the MongoDB manager
func (df *DatabaseFactory) GetMongoDB() *MongoManager {
	return df.mongo
}

// HealthCheck performs health checks on all configured databases
func (df *DatabaseFactory) HealthCheck(ctx context.Context) error {
	var errors []error

	if df.postgres != nil {
		if err := df.postgres.HealthCheck(ctx); err != nil {
			errors = append(errors, fmt.Errorf("PostgreSQL health check failed: %w", err))
		}
	}

	if df.mongo != nil {
		if err := df.mongo.HealthCheck(ctx); err != nil {
			errors = append(errors, fmt.Errorf("MongoDB health check failed: %w", err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("database health checks failed: %v", errors)
	}

	return nil
}

// Close closes all database connections
func (df *DatabaseFactory) Close() error {
	var errors []error

	if df.postgres != nil {
		if err := df.postgres.Close(); err != nil {
			errors = append(errors, fmt.Errorf("PostgreSQL close failed: %w", err))
		}
	}

	if df.mongo != nil {
		if err := df.mongo.Close(); err != nil {
			errors = append(errors, fmt.Errorf("MongoDB close failed: %w", err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("database close errors: %v", errors)
	}

	return nil
}

// GetConnectionStats returns connection statistics for all databases
func (df *DatabaseFactory) GetConnectionStats() map[DatabaseType]map[string]int {
	stats := make(map[DatabaseType]map[string]int)

	if df.postgres != nil {
		active, idle, max := df.postgres.GetConnectionStats()
		stats[DatabaseTypePostgreSQL] = map[string]int{
			"active": active,
			"idle":   idle,
			"max":    max,
		}
	}

	if df.mongo != nil {
		active, idle, max := df.mongo.GetConnectionStats()
		stats[DatabaseTypeMongoDB] = map[string]int{
			"active": active,
			"idle":   idle,
			"max":    max,
		}
	}

	return stats
}

// IsPostgreSQLAvailable returns true if PostgreSQL is configured and available
func (df *DatabaseFactory) IsPostgreSQLAvailable() bool {
	return df.postgres != nil
}

// IsMongoDBAvailable returns true if MongoDB is configured and available
func (df *DatabaseFactory) IsMongoDBAvailable() bool {
	return df.mongo != nil
}
