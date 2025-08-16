package database

import (
	"context"
	"fmt"
	"time"

	"github.com/duccv/go-clean-template/config"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type PostgresDB struct {
	config    *config.PostgresConfig
	readPool  *pgxpool.Pool
	writePool *pgxpool.Pool
	logger    *zap.Logger
}

func NewPostgresDB(config *config.PostgresConfig) *PostgresDB {
	if config.ConnectTimeout <= 0 {
		config.ConnectTimeout = 30
	}
	return &PostgresDB{
		config: config,
		logger: zap.L(),
	}
}

func (p *PostgresDB) Connect() error {
	p.logger.Info("Starting PostgreSQL connection",
		zap.String("host", p.config.Host),
		zap.Int("port", p.config.Port),
		zap.String("database", p.config.Database),
		zap.String("username", p.config.Username))

	ctx, cannel := context.WithTimeout(
		context.Background(),
		time.Duration(p.config.ConnectTimeout)*time.Second,
	)
	defer cannel()

	// Tạo write connection
	writeHost := p.config.WriteHost
	writePort := p.config.WritePort
	if writeHost == "" {
		writeHost = p.config.Host
		writePort = p.config.Port
	}

	p.logger.Info("Connecting to write pool",
		zap.String("write_host", writeHost),
		zap.Int("write_port", writePort))

	writeDSN := p.buildPgxDSN(writeHost, writePort)
	writePoolConfig, err := pgxpool.ParseConfig(writeDSN)
	if err != nil {
		p.logger.Error("Failed to parse write pool config",
			zap.String("write_host", writeHost),
			zap.Int("write_port", writePort),
			zap.Error(err))
		return fmt.Errorf("failed to parse write pool config: %w", err)
	}

	p.configurePool(writePoolConfig)
	p.writePool, err = pgxpool.NewWithConfig(ctx, writePoolConfig)
	if err != nil {
		p.logger.Error("Failed to create write pool",
			zap.String("write_host", writeHost),
			zap.Int("write_port", writePort),
			zap.Error(err))
		return fmt.Errorf("failed to create write pool: %w", err)
	}

	p.logger.Debug("Write pool created successfully",
		zap.String("write_host", writeHost),
		zap.Int("write_port", writePort))

	readHost := p.config.ReadHost
	readPort := p.config.ReadPort
	if readHost == "" {
		readHost = p.config.Host
		readPort = p.config.Port
	}

	p.logger.Debug("Connecting to read pool",
		zap.String("read_host", readHost),
		zap.Int("read_port", readPort))

	readDSN := p.buildPgxDSN(readHost, readPort)
	readPoolConfig, err := pgxpool.ParseConfig(readDSN)
	if err != nil {
		p.logger.Error("Failed to parse read pool config",
			zap.String("read_host", readHost),
			zap.Int("read_port", readPort),
			zap.Error(err))
		p.writePool.Close()
		return fmt.Errorf("failed to parse read pool config: %w", err)
	}

	p.configurePool(readPoolConfig)
	p.readPool, err = pgxpool.NewWithConfig(ctx, readPoolConfig)
	if err != nil {
		p.logger.Error("Failed to create read pool",
			zap.String("read_host", readHost),
			zap.Int("read_port", readPort),
			zap.Error(err))
		p.writePool.Close()
		return fmt.Errorf("failed to create read pool: %w", err)
	}

	p.logger.Info("Read pool created successfully",
		zap.String("read_host", readHost),
		zap.Int("read_port", readPort))

	// Ping write pool
	p.logger.Debug("Pinging write pool")
	if err := p.writePool.Ping(ctx); err != nil {
		p.logger.Error("Failed to ping write pool",
			zap.String("write_host", writeHost),
			zap.Int("write_port", writePort),
			zap.Error(err))
		p.Close()
		return fmt.Errorf("failed to ping write pool: %w", err)
	}
	p.logger.Debug("Write pool ping successful")

	// Ping read pool
	p.logger.Debug("Pinging read pool")
	if err := p.readPool.Ping(ctx); err != nil {
		p.logger.Error("Failed to ping read pool",
			zap.String("read_host", readHost),
			zap.Int("read_port", readPort),
			zap.Error(err))
		p.Close()
		return fmt.Errorf("failed to ping read pool: %w", err)
	}
	p.logger.Debug("Read pool ping successful")

	p.logger.Info("Successfully connected to PostgreSQL",
		zap.String("write_host", writeHost),
		zap.Int("write_port", writePort),
		zap.String("read_host", readHost),
		zap.Int("read_port", readPort))
	return nil
}

func (p *PostgresDB) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	p.logger.Debug("Starting database ping")

	if p.writePool != nil {
		if err := p.writePool.Ping(ctx); err != nil {
			p.logger.Error("Write pool ping failed", zap.Error(err))
			return fmt.Errorf("write pool ping failed: %w", err)
		}
		p.logger.Debug("Write pool ping successful")
	} else {
		p.logger.Warn("Write pool is nil during ping")
	}

	if p.readPool != nil {
		if err := p.readPool.Ping(ctx); err != nil {
			p.logger.Error("Read pool ping failed", zap.Error(err))
			return fmt.Errorf("read pool ping failed: %w", err)
		}
		p.logger.Debug("Read pool ping successful")
	} else {
		p.logger.Warn("Read pool is nil during ping")
	}

	p.logger.Debug("Database ping completed successfully")
	return nil
}

func (p *PostgresDB) GetReadConnection() interface{} {
	return p.readPool
}

func (p *PostgresDB) GetWriteConnection() interface{} {
	return p.writePool
}

func (p *PostgresDB) IsConnected() bool {
	isConnected := p.writePool != nil && p.readPool != nil
	p.logger.Debug("Checking database connection status",
		zap.Bool("is_connected", isConnected),
		zap.Bool("write_pool_exists", p.writePool != nil),
		zap.Bool("read_pool_exists", p.readPool != nil))
	return isConnected
}

func (p *PostgresDB) HealthCheck() map[string]error {
	p.logger.Debug("Starting database health check")
	result := make(map[string]error)
	ctx := context.Background()

	if p.writePool != nil {
		err := p.writePool.Ping(ctx)
		result["write_pool"] = err
		if err != nil {
			p.logger.Error("Write pool health check failed", zap.Error(err))
		} else {
			p.logger.Debug("Write pool health check passed")
		}
	} else {
		err := fmt.Errorf("write pool not initialized")
		result["write_pool"] = err
		p.logger.Warn("Write pool not initialized during health check")
	}

	if p.readPool != nil {
		err := p.readPool.Ping(ctx)
		result["read_pool"] = err
		if err != nil {
			p.logger.Error("Read pool health check failed", zap.Error(err))
		} else {
			p.logger.Debug("Read pool health check passed")
		}
	} else {
		err := fmt.Errorf("read pool not initialized")
		result["read_pool"] = err
		p.logger.Warn("Read pool not initialized during health check")
	}

	p.logger.Debug("Database health check completed", zap.Any("results", result))
	return result
}

func (p *PostgresDB) buildPgxDSN(host string, port int) string {
	sslMode := p.config.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		p.config.Username, p.config.Password, host, port, p.config.Database, sslMode)

	p.logger.Debug("Built PostgreSQL DSN",
		zap.String("host", host),
		zap.Int("port", port),
		zap.String("database", p.config.Database),
		zap.String("username", p.config.Username),
		zap.String("ssl_mode", sslMode))

	return dsn
}

func (p *PostgresDB) GetType() DatabaseType {
	return PostgreSQL
}

func (p *PostgresDB) Close() error {
	p.logger.Info("Closing PostgreSQL connections")

	if p.writePool != nil {
		p.logger.Debug("Closing write pool")
		p.writePool.Close()
		p.logger.Debug("Write pool closed")
	}

	if p.readPool != nil {
		p.logger.Debug("Closing read pool")
		p.readPool.Close()
		p.logger.Debug("Read pool closed")
	}

	p.logger.Info("PostgreSQL connections closed successfully")
	return nil
}

func (p *PostgresDB) configurePool(config *pgxpool.Config) {
	p.logger.Debug("Configuring connection pool",
		zap.Int32("max_conns", p.config.MaxConns),
		zap.Int32("min_conns", p.config.MinConns),
		zap.Int("conn_max_idle_time_minutes", p.config.ConnMaxIdleTime),
		zap.Int("conn_max_lifetime_hours", p.config.ConnMaxLifetime),
		zap.Int("health_check_period_minutes", p.config.HealthCheckPeriod))

	if p.config.MaxConns != 0 {
		config.MaxConns = p.config.MaxConns
	}

	if p.config.MinConns != 0 {
		config.MinConns = p.config.MinConns
	}

	if p.config.ConnMaxIdleTime != 0 {
		config.MaxConnIdleTime = time.Duration(p.config.ConnMaxIdleTime) * time.Minute
	}

	if p.config.ConnMaxLifetime != 0 {
		config.MaxConnLifetime = time.Duration(p.config.ConnMaxLifetime) * time.Hour
	}

	if p.config.HealthCheckPeriod != 0 {
		config.HealthCheckPeriod = time.Duration(p.config.HealthCheckPeriod) * time.Minute
	}

	p.logger.Debug("Connection pool configuration completed")
}
