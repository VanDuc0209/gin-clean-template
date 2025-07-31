package database

import (
	"context"
	"fmt"
	"time"

	"github.com/duccv/go-clean-template/config"
	"github.com/duccv/go-clean-template/pkg/metrics"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.uber.org/zap"
)

// MongoManager manages MongoDB database connections and operations
type MongoManager struct {
	client  *mongo.Client
	config  config.MongoConfig
	logger  *zap.Logger
	metrics *metrics.MetricsCollector
}

// NewMongoManager creates a new MongoDB manager
func NewMongoManager(
	cfg config.MongoConfig,
	logger *zap.Logger,
	metrics *metrics.MetricsCollector,
) (*MongoManager, error) {
	manager := &MongoManager{
		config:  cfg,
		logger:  logger,
		metrics: metrics,
	}

	if err := manager.connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	// Start connection monitoring
	go manager.monitorConnections()

	return manager, nil
}

// connect establishes a connection to MongoDB
func (mm *MongoManager) connect() error {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		time.Duration(mm.config.ConnectTimeout)*time.Second,
	)
	defer cancel()

	// Build connection options
	clientOptions := options.Client().ApplyURI(mm.config.URI)

	// Set connection pool options
	clientOptions.SetMaxPoolSize(uint64(mm.config.MaxPoolSize))
	clientOptions.SetMinPoolSize(uint64(mm.config.MinPoolSize))
	clientOptions.SetMaxConnIdleTime(time.Duration(mm.config.SocketTimeout) * time.Second)

	// Set server selection timeout
	clientOptions.SetServerSelectionTimeout(time.Duration(mm.config.ConnectTimeout) * time.Second)

	// Connect to MongoDB
	client, err := mongo.Connect(clientOptions)
	if err != nil {
		return fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	// Test the connection
	if err := client.Ping(ctx, nil); err != nil {
		return fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	mm.client = client
	mm.logger.Info("Successfully connected to MongoDB",
		zap.String("uri", mm.config.URI),
		zap.String("database", mm.config.Database),
	)

	return nil
}

// GetClient returns the underlying MongoDB client
func (mm *MongoManager) GetClient() *mongo.Client {
	return mm.client
}

// GetDatabase returns a database instance
func (mm *MongoManager) GetDatabase() *mongo.Database {
	return mm.client.Database(mm.config.Database)
}

// GetCollection returns a collection instance
func (mm *MongoManager) GetCollection(collectionName string) *mongo.Collection {
	return mm.client.Database(mm.config.Database).Collection(collectionName)
}

// Ping checks if the database is accessible
func (mm *MongoManager) Ping(ctx context.Context) error {
	start := time.Now()
	err := mm.client.Ping(ctx, nil)
	duration := time.Since(start)

	mm.metrics.RecordDatabaseOperation(
		mm.config.Database,
		"ping",
		"",
		duration,
		err,
	)

	if err != nil {
		mm.logger.Error("MongoDB ping failed", zap.Error(err))
		return fmt.Errorf("MongoDB ping failed: %w", err)
	}

	mm.logger.Debug("MongoDB ping successful", zap.Duration("duration", duration))
	return nil
}

// HealthCheck performs a comprehensive health check
func (mm *MongoManager) HealthCheck(ctx context.Context) error {
	// Check basic connectivity
	if err := mm.Ping(ctx); err != nil {
		return err
	}

	// Check if we can execute a simple command
	start := time.Now()
	err := mm.client.Database(mm.config.Database).
		RunCommand(ctx, map[string]interface{}{"ping": 1}).
		Err()
	duration := time.Since(start)

	mm.metrics.RecordDatabaseOperation(
		mm.config.Database,
		"health_check",
		"",
		duration,
		err,
	)

	if err != nil {
		mm.logger.Error("MongoDB health check failed", zap.Error(err))
		return fmt.Errorf("MongoDB health check failed: %w", err)
	}

	mm.logger.Debug("MongoDB health check passed", zap.Duration("duration", duration))
	return nil
}

// GetConnectionStats returns current connection pool statistics
func (mm *MongoManager) GetConnectionStats() (active, idle, max int) {
	// MongoDB doesn't expose connection pool stats through the driver
	// We'll return placeholder values for now
	// In a real implementation, you might want to use MongoDB's serverStatus command
	return 0, 0, mm.config.MaxPoolSize
}

// monitorConnections periodically monitors connection pool statistics
func (mm *MongoManager) monitorConnections() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// MongoDB doesn't expose connection pool stats through the driver
			// We could implement this by running serverStatus command
			mm.logger.Debug("MongoDB connection monitoring",
				zap.Int("max_pool_size", mm.config.MaxPoolSize),
				zap.Int("min_pool_size", mm.config.MinPoolSize),
			)
		}
	}
}

// Close closes the MongoDB connection
func (mm *MongoManager) Close() error {
	if mm.client != nil {
		mm.logger.Info("Closing MongoDB connection")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return mm.client.Disconnect(ctx)
	}
	return nil
}

// WithTransaction executes a function within a MongoDB transaction
func (mm *MongoManager) WithTransaction(
	ctx context.Context,
	fn func(sessCtxctx context.Context) error,
) error {
	start := time.Now()

	session, err := mm.client.StartSession()
	if err != nil {
		mm.metrics.RecordDatabaseOperation(
			mm.config.Database,
			"start_session",
			"",
			time.Since(start),
			err,
		)
		return fmt.Errorf("failed to start session: %w", err)
	}
	defer session.EndSession(ctx)

	wrappedFn := func(sessCtx context.Context) (interface{}, error) {
		return nil, fn(sessCtx)
	}

	_, err = session.WithTransaction(ctx, wrappedFn)
	_, err = session.WithTransaction(ctx, wrappedFn)
	duration := time.Since(start)

	mm.metrics.RecordDatabaseOperation(
		mm.config.Database,
		"transaction",
		"",
		duration,
		err,
	)

	if err != nil {
		mm.logger.Error("MongoDB transaction failed", zap.Error(err))
	} else {
		mm.logger.Debug("MongoDB transaction completed", zap.Duration("duration", duration))
	}

	return err
}

// InsertOne inserts a single document
func (mm *MongoManager) InsertOne(
	ctx context.Context,
	collectionName string,
	document interface{},
) (*mongo.InsertOneResult, error) {
	start := time.Now()
	result, err := mm.GetCollection(collectionName).InsertOne(ctx, document)
	duration := time.Since(start)

	mm.metrics.RecordDatabaseOperation(
		mm.config.Database,
		"insert_one",
		collectionName,
		duration,
		err,
	)

	if err != nil {
		mm.logger.Error("MongoDB insert one failed",
			zap.String("collection", collectionName),
			zap.Error(err),
			zap.Duration("duration", duration),
		)
	} else {
		mm.logger.Debug("MongoDB insert one successful",
			zap.String("collection", collectionName),
			zap.Duration("duration", duration),
		)
	}

	return result, err
}

// FindOne finds a single document
func (mm *MongoManager) FindOne(
	ctx context.Context,
	collectionName string,
	filter interface{},
	result interface{},
) error {
	start := time.Now()
	err := mm.GetCollection(collectionName).FindOne(ctx, filter).Decode(result)
	duration := time.Since(start)

	mm.metrics.RecordDatabaseOperation(
		mm.config.Database,
		"find_one",
		collectionName,
		duration,
		err,
	)

	if err != nil {
		mm.logger.Error("MongoDB find one failed",
			zap.String("collection", collectionName),
			zap.Error(err),
			zap.Duration("duration", duration),
		)
	} else {
		mm.logger.Debug("MongoDB find one successful",
			zap.String("collection", collectionName),
			zap.Duration("duration", duration),
		)
	}

	return err
}

// UpdateOne updates a single document
func (mm *MongoManager) UpdateOne(
	ctx context.Context,
	collectionName string,
	filter, update interface{},
) (*mongo.UpdateResult, error) {
	start := time.Now()
	result, err := mm.GetCollection(collectionName).UpdateOne(ctx, filter, update)
	duration := time.Since(start)

	mm.metrics.RecordDatabaseOperation(
		mm.config.Database,
		"update_one",
		collectionName,
		duration,
		err,
	)

	if err != nil {
		mm.logger.Error("MongoDB update one failed",
			zap.String("collection", collectionName),
			zap.Error(err),
			zap.Duration("duration", duration),
		)
	} else {
		mm.logger.Debug("MongoDB update one successful",
			zap.String("collection", collectionName),
			zap.Duration("duration", duration),
		)
	}

	return result, err
}

// DeleteOne deletes a single document
func (mm *MongoManager) DeleteOne(
	ctx context.Context,
	collectionName string,
	filter interface{},
) (*mongo.DeleteResult, error) {
	start := time.Now()
	result, err := mm.GetCollection(collectionName).DeleteOne(ctx, filter)
	duration := time.Since(start)

	mm.metrics.RecordDatabaseOperation(
		mm.config.Database,
		"delete_one",
		collectionName,
		duration,
		err,
	)

	if err != nil {
		mm.logger.Error("MongoDB delete one failed",
			zap.String("collection", collectionName),
			zap.Error(err),
			zap.Duration("duration", duration),
		)
	} else {
		mm.logger.Debug("MongoDB delete one successful",
			zap.String("collection", collectionName),
			zap.Duration("duration", duration),
		)
	}

	return result, err
}
