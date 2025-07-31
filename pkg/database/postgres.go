package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/duccv/go-clean-template/config"
	"github.com/duccv/go-clean-template/pkg/metrics"
	"go.uber.org/zap"
)

// PostgreSQLManager manages PostgreSQL database connections and operations
type PostgreSQLManager struct {
	db      *sql.DB
	config  config.PostgresConfig
	logger  *zap.Logger
	metrics *metrics.MetricsCollector
}

// NewPostgreSQLManager creates a new PostgreSQL manager
func NewPostgreSQLManager(
	cfg config.PostgresConfig,
	logger *zap.Logger,
	metrics *metrics.MetricsCollector,
) (*PostgreSQLManager, error) {
	manager := &PostgreSQLManager{
		config:  cfg,
		logger:  logger,
		metrics: metrics,
	}

	if err := manager.connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	// Start connection pool monitoring
	go manager.monitorConnections()

	return manager, nil
}

// connect establishes a connection to PostgreSQL
func (pm *PostgreSQLManager) connect() error {
	dsn := pm.buildDSN()

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}

	// Configure connection pool
	if pm.config.MaxIdleConns != 0 {
		db.SetMaxOpenConns(pm.config.MaxOpenConns)
	}
	if pm.config.MaxOpenConns != 0 {
		db.SetMaxIdleConns(pm.config.MaxIdleConns)
	}
	if pm.config.ConnMaxLifetime != 0 {
		db.SetConnMaxLifetime(time.Duration(pm.config.ConnMaxLifetime) * time.Second)
	}
	if pm.config.ConnMaxIdleTime != 0 {
		db.SetConnMaxIdleTime(time.Duration(pm.config.ConnMaxIdleTime) * time.Second)
	}

	// Test the connection
	ctx, cancel := context.WithTimeout(
		context.Background(),
		time.Duration(pm.config.ConnectionTimeout)*time.Second,
	)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	pm.db = db
	pm.logger.Info("Successfully connected to PostgreSQL",
		zap.String("host", pm.config.Host),
		zap.Int("port", pm.config.Port),
		zap.String("database", pm.config.Database),
	)

	return nil
}

// buildDSN builds the PostgreSQL connection string
func (pm *PostgreSQLManager) buildDSN() string {
	if pm.config.ConnectionString != "" {
		return pm.config.ConnectionString
	}

	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		pm.config.Host,
		pm.config.Port,
		pm.config.User,
		pm.config.Password,
		pm.config.Database,
		pm.config.SSLMode,
	)
}

// GetDB returns the underlying sql.DB instance
func (pm *PostgreSQLManager) GetDB() *sql.DB {
	return pm.db
}

// Ping checks if the database is accessible
func (pm *PostgreSQLManager) Ping(ctx context.Context) error {
	start := time.Now()
	err := pm.db.PingContext(ctx)
	duration := time.Since(start)

	pm.metrics.RecordDatabaseOperation(
		pm.config.Database,
		"ping",
		"",
		duration,
		err,
	)

	if err != nil {
		pm.logger.Error("Database ping failed", zap.Error(err))
		return fmt.Errorf("database ping failed: %w", err)
	}

	pm.logger.Debug("Database ping successful", zap.Duration("duration", duration))
	return nil
}

// HealthCheck performs a comprehensive health check
func (pm *PostgreSQLManager) HealthCheck(ctx context.Context) error {
	// Check basic connectivity
	if err := pm.Ping(ctx); err != nil {
		return err
	}

	// Check if we can execute a simple query
	start := time.Now()
	var result int
	err := pm.db.QueryRowContext(ctx, "SELECT 1").Scan(&result)
	duration := time.Since(start)

	pm.metrics.RecordDatabaseOperation(
		pm.config.Database,
		"health_check",
		"",
		duration,
		err,
	)

	if err != nil {
		pm.logger.Error("Database health check failed", zap.Error(err))
		return fmt.Errorf("database health check failed: %w", err)
	}

	pm.logger.Debug("Database health check passed", zap.Duration("duration", duration))
	return nil
}

// GetConnectionStats returns current connection pool statistics
func (pm *PostgreSQLManager) GetConnectionStats() (active, idle, max int) {
	stats := pm.db.Stats()
	return stats.InUse, stats.Idle, stats.MaxOpenConnections
}

// monitorConnections periodically monitors connection pool statistics
func (pm *PostgreSQLManager) monitorConnections() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			stats := pm.db.Stats()
			pm.metrics.RecordDatabaseConnections(
				pm.config.Database,
				stats.InUse,
				stats.Idle,
				stats.MaxOpenConnections,
			)

			pm.logger.Debug("Database connection pool stats",
				zap.Int("active", stats.InUse),
				zap.Int("idle", stats.Idle),
				zap.Int("max", stats.MaxOpenConnections),
				zap.Int64("wait_count", stats.WaitCount),
				zap.Duration("wait_duration", stats.WaitDuration),
			)
		}
	}
}

// Close closes the database connection
func (pm *PostgreSQLManager) Close() error {
	if pm.db != nil {
		pm.logger.Info("Closing PostgreSQL connection")
		return pm.db.Close()
	}
	return nil
}

// ExecContext executes a query without returning rows
func (pm *PostgreSQLManager) ExecContext(
	ctx context.Context,
	query string,
	args ...interface{},
) (sql.Result, error) {
	start := time.Now()
	result, err := pm.db.ExecContext(ctx, query, args...)
	duration := time.Since(start)

	pm.metrics.RecordDatabaseOperation(
		pm.config.Database,
		"exec",
		"",
		duration,
		err,
	)

	if err != nil {
		pm.logger.Error("Database exec failed",
			zap.String("query", query),
			zap.Error(err),
			zap.Duration("duration", duration),
		)
	} else {
		pm.logger.Debug("Database exec successful",
			zap.String("query", query),
			zap.Duration("duration", duration),
		)
	}

	return result, err
}

// QueryContext executes a query that returns rows
func (pm *PostgreSQLManager) QueryContext(
	ctx context.Context,
	query string,
	args ...interface{},
) (*sql.Rows, error) {
	start := time.Now()
	rows, err := pm.db.QueryContext(ctx, query, args...)
	duration := time.Since(start)

	pm.metrics.RecordDatabaseOperation(
		pm.config.Database,
		"query",
		"",
		duration,
		err,
	)

	if err != nil {
		pm.logger.Error("Database query failed",
			zap.String("query", query),
			zap.Error(err),
			zap.Duration("duration", duration),
		)
	} else {
		pm.logger.Debug("Database query successful",
			zap.String("query", query),
			zap.Duration("duration", duration),
		)
	}

	return rows, err
}

// QueryRowContext executes a query that returns a single row
func (pm *PostgreSQLManager) QueryRowContext(
	ctx context.Context,
	query string,
	args ...interface{},
) *sql.Row {
	start := time.Now()
	row := pm.db.QueryRowContext(ctx, query, args...)
	duration := time.Since(start)

	pm.metrics.RecordDatabaseOperation(
		pm.config.Database,
		"query_row",
		"",
		duration,
		nil, // QueryRow doesn't return an error immediately
	)

	pm.logger.Debug("Database query row executed",
		zap.String("query", query),
		zap.Duration("duration", duration),
	)

	return row
}

// BeginTx starts a new transaction
func (pm *PostgreSQLManager) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	start := time.Now()
	tx, err := pm.db.BeginTx(ctx, opts)
	duration := time.Since(start)

	pm.metrics.RecordDatabaseOperation(
		pm.config.Database,
		"begin_tx",
		"",
		duration,
		err,
	)

	if err != nil {
		pm.logger.Error("Database transaction begin failed", zap.Error(err))
	} else {
		pm.logger.Debug("Database transaction begun", zap.Duration("duration", duration))
	}

	return tx, err
}
