// mongodb.go - MongoDB Database Implementation với Driver v2
package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/duccv/go-clean-template/config"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
	"go.uber.org/zap"
)

// MongoDB implement Database interface cho MongoDB Driver v2
type MongoDB struct {
	config      *config.MongoConfig
	readClient  *mongo.Client
	writeClient *mongo.Client
	readDB      *mongo.Database
	writeDB     *mongo.Database
	logger      *zap.Logger
}

func NewMongoDB(config *config.MongoConfig) *MongoDB {
	if config.ConnectTimeout <= 0 {
		config.ConnectTimeout = 30
	}
	return &MongoDB{
		config: config,
		logger: zap.L(),
	}
}

func (m *MongoDB) Connect() error {
	m.logger.Info("Starting MongoDB connection",
		zap.String("deployment_type", m.config.Type),
		zap.String("database", m.config.Database),
		zap.String("auth_source", m.config.AuthSource),
		zap.Int("connect_timeout_seconds", m.config.ConnectTimeout))

	ctx, cancel := context.WithTimeout(
		context.Background(),
		time.Duration(m.config.ConnectTimeout)*time.Second,
	)
	defer cancel()

	// Set default deployment type
	if m.config.Type == "" {
		m.config.Type = string(MongoSingle)
		m.logger.Debug("Set default deployment type to single")
	}

	m.logger.Info("Connecting to MongoDB",
		zap.String("deployment_type", m.config.Type))

	switch MongoDeployment(m.config.Type) {
	case MongoSingle:
		return m.connectSingle(ctx)
	case MongoReplicaSet:
		return m.connectReplicaSet(ctx)
	case MongoSharded:
		return m.connectSharded(ctx)
	default:
		err := fmt.Errorf("unsupported MongoDB deployment: %s", m.config.Type)
		m.logger.Error("Unsupported MongoDB deployment type",
			zap.String("deployment_type", m.config.Type))
		return err
	}
}

// connectSingle kết nối đến MongoDB single instance
func (m *MongoDB) connectSingle(ctx context.Context) error {
	m.logger.Info("Connecting to MongoDB single instance")

	writeHost, writePort := m.getWriteHostPort()
	readHost, readPort := m.getReadHostPort()

	m.logger.Debug("Host configuration",
		zap.String("write_host", writeHost),
		zap.Int("write_port", writePort),
		zap.String("read_host", readHost),
		zap.Int("read_port", readPort),
		zap.String("auth_source", m.config.AuthSource))

	// Tạo write client
	m.logger.Debug("Creating write client")
	writeURI := m.buildMongoURI([]string{writeHost}, []int{writePort}, "", m.config.AuthSource)
	writeClient, err := m.createClient(ctx, writeURI, "write")
	if err != nil {
		m.logger.Error("Failed to create write client",
			zap.String("write_host", writeHost),
			zap.Int("write_port", writePort),
			zap.Error(err))
		return fmt.Errorf("failed to create write client: %w", err)
	}
	m.writeClient = writeClient
	m.writeDB = writeClient.Database(m.config.Database)
	m.logger.Info("Write client created successfully",
		zap.String("write_host", writeHost),
		zap.Int("write_port", writePort))

	// Tạo read client (có thể cùng instance hoặc khác)
	if readHost == writeHost && readPort == writePort {
		m.logger.Debug("Using same client for read and write (single instance)")
		m.readClient = writeClient
		m.readDB = m.writeDB
	} else {
		m.logger.Debug("Creating separate read client")
		readURI := m.buildMongoURI([]string{readHost}, []int{readPort}, "", m.config.AuthSource)
		readClient, err := m.createClient(ctx, readURI, "read")
		if err != nil {
			m.logger.Error("Failed to create read client",
				zap.String("read_host", readHost),
				zap.Int("read_port", readPort),
				zap.Error(err))
			m.writeClient.Disconnect(ctx)
			return fmt.Errorf("failed to create read client: %w", err)
		}
		m.readClient = readClient
		m.readDB = readClient.Database(m.config.Database)
		m.logger.Info("Read client created successfully",
			zap.String("read_host", readHost),
			zap.Int("read_port", readPort))
	}

	m.logger.Debug("Testing connections")
	return m.testConnections(ctx, writeHost, writePort, readHost, readPort)
}

// connectReplicaSet kết nối đến MongoDB replica set
func (m *MongoDB) connectReplicaSet(ctx context.Context) error {
	m.logger.Info("Connecting to MongoDB replica set",
		zap.String("replica_set_name", m.config.ReplicaSetName),
		zap.Strings("hosts", m.config.Hosts),
		zap.String("auth_source", m.config.AuthSource))

	if len(m.config.Hosts) == 0 {
		err := fmt.Errorf("replica set hosts not configured")
		m.logger.Error("Replica set hosts not configured")
		return err
	}

	if m.config.ReplicaSetName == "" {
		err := fmt.Errorf("replica set name not configured")
		m.logger.Error("Replica set name not configured")
		return err
	}

	ports := m.config.Ports
	if len(ports) == 0 {
		// Default port cho tất cả replica set hosts
		ports = make([]int, len(m.config.Hosts))
		for i := range ports {
			ports[i] = 27017
		}
		m.logger.Debug("Using default port 27017 for all hosts")
	} else if len(ports) != len(m.config.Hosts) {
		err := fmt.Errorf("number of ports must match number of hosts")
		m.logger.Error("Port count mismatch",
			zap.Int("hosts_count", len(m.config.Hosts)),
			zap.Int("ports_count", len(ports)))
		return err
	}

	// Tạo write client với primary preference
	m.logger.Debug("Creating replica set write client")
	writeURI := m.buildMongoURI(m.config.Hosts, ports, m.config.ReplicaSetName, m.config.AuthSource)
	writeClient, err := m.createClient(ctx, writeURI, "write")
	if err != nil {
		m.logger.Error("Failed to create replica set write client",
			zap.Strings("hosts", m.config.Hosts),
			zap.String("replica_set_name", m.config.ReplicaSetName),
			zap.Error(err))
		return fmt.Errorf("failed to create replica set write client: %w", err)
	}
	m.writeClient = writeClient
	m.writeDB = writeClient.Database(m.config.Database)
	m.logger.Info("Replica set write client created successfully")

	// Tạo read client với secondary preferred
	m.logger.Debug("Creating replica set read client")
	readClient, err := m.createClient(ctx, writeURI, "read")
	if err != nil {
		m.logger.Error("Failed to create replica set read client",
			zap.Strings("hosts", m.config.Hosts),
			zap.String("replica_set_name", m.config.ReplicaSetName),
			zap.Error(err))
		m.writeClient.Disconnect(ctx)
		return fmt.Errorf("failed to create replica set read client: %w", err)
	}
	m.readClient = readClient
	m.readDB = readClient.Database(m.config.Database)
	m.logger.Info("Replica set read client created successfully")

	// Test connections
	m.logger.Debug("Testing replica set connections")
	if err := m.testReplicaSetConnections(ctx); err != nil {
		m.logger.Error("Replica set connection test failed", zap.Error(err))
		m.Close()
		return err
	}

	m.logger.Info("Successfully connected to MongoDB Replica Set",
		zap.String("replica_set_name", m.config.ReplicaSetName),
		zap.Strings("hosts", m.config.Hosts))
	return nil
}

// connectSharded kết nối đến MongoDB sharded cluster
func (m *MongoDB) connectSharded(ctx context.Context) error {
	m.logger.Info("Connecting to MongoDB sharded cluster",
		zap.Strings("mongos_hosts", m.config.Hosts),
		zap.String("auth_source", m.config.AuthSource))

	if len(m.config.Hosts) == 0 {
		err := fmt.Errorf("mongos hosts not configured")
		m.logger.Error("Mongos hosts not configured")
		return err
	}

	ports := m.config.Ports
	if len(ports) == 0 {
		// Default port cho tất cả mongos hosts
		ports = make([]int, len(m.config.Hosts))
		for i := range ports {
			ports[i] = 27017
		}
		m.logger.Debug("Using default port 27017 for all mongos hosts")
	} else if len(ports) != len(m.config.Ports) {
		err := fmt.Errorf("number of mongos ports must match number of mongos hosts")
		m.logger.Error("Mongos port count mismatch",
			zap.Int("hosts_count", len(m.config.Hosts)),
			zap.Int("ports_count", len(ports)))
		return err
	}

	// Tạo URI với multiple mongos instances
	m.logger.Debug("Building mongos URI")
	mongosURI := m.buildMongoURI(m.config.Hosts, ports, "", m.config.AuthSource)

	// Tạo write client
	m.logger.Debug("Creating sharded write client")
	writeClient, err := m.createClient(ctx, mongosURI, "write")
	if err != nil {
		m.logger.Error("Failed to create sharded write client",
			zap.Strings("mongos_hosts", m.config.Hosts),
			zap.Error(err))
		return fmt.Errorf("failed to create sharded write client: %w", err)
	}
	m.writeClient = writeClient
	m.writeDB = writeClient.Database(m.config.Database)
	m.logger.Info("Sharded write client created successfully")

	// Tạo read client với read preference
	m.logger.Debug("Creating sharded read client")
	readClient, err := m.createClient(ctx, mongosURI, "read")
	if err != nil {
		m.logger.Error("Failed to create sharded read client",
			zap.Strings("mongos_hosts", m.config.Hosts),
			zap.Error(err))
		m.writeClient.Disconnect(ctx)
		return fmt.Errorf("failed to create sharded read client: %w", err)
	}
	m.readClient = readClient
	m.readDB = readClient.Database(m.config.Database)
	m.logger.Info("Sharded read client created successfully")

	// Test connections
	m.logger.Debug("Testing sharded cluster connections")
	if err := m.testShardedConnections(ctx); err != nil {
		m.logger.Error("Sharded cluster connection test failed", zap.Error(err))
		m.Close()
		return err
	}

	m.logger.Info("Successfully connected to MongoDB Sharded Cluster",
		zap.Strings("mongos_hosts", m.config.Hosts))
	return nil
}

func (m *MongoDB) getWriteHostPort() (string, int) {
	writeHost := m.config.WriteHost
	writePort := m.config.WritePort
	if writeHost == "" {
		writeHost = m.config.Hosts[0]
		writePort = m.config.Ports[0]
	}
	if writePort == 0 {
		writePort = 27017
	}
	return writeHost, writePort
}

func (m *MongoDB) getReadHostPort() (string, int) {
	readHost := m.config.ReadHost
	readPort := m.config.ReadPort
	if readHost == "" {
		readHost = m.config.Hosts[0]
		readPort = m.config.Ports[0]
	}
	if readPort == 0 {
		readPort = 27017
	}
	return readHost, readPort
}

func (m *MongoDB) buildMongoURI(
	hosts []string,
	ports []int,
	replicaSetName string,
	authSource string,
) string {
	if m.config.URI != "" {
		return m.config.URI
	}
	var hostPorts []string

	for i, host := range hosts {
		port := 27017 // default port
		if i < len(ports) {
			port = ports[i]
		}
		hostPorts = append(hostPorts, fmt.Sprintf("%s:%d", host, port))
	}

	uri := fmt.Sprintf("mongodb://%s:%s@%s/%s",
		m.config.Username, m.config.Password,
		strings.Join(hostPorts, ","), m.config.Database)

	// Build query parameters
	var queryParams []string

	// Thêm replica set name nếu có
	if replicaSetName != "" {
		queryParams = append(queryParams, "replicaSet="+replicaSetName)
	}

	// Thêm authSource nếu có
	if authSource != "" {
		queryParams = append(queryParams, "authSource="+authSource)
	} else if m.config.AuthSource != "" {
		// Use config default if not provided
		queryParams = append(queryParams, "authSource="+m.config.AuthSource)
	}

	// Add query parameters to URI if any exist
	if len(queryParams) > 0 {
		uri += "?" + strings.Join(queryParams, "&")
	}

	m.logger.Info("Built MongoDB URI",
		zap.Strings("hosts", hosts),
		zap.String("replica_set_name", replicaSetName),
		zap.String("auth_source", authSource),
		zap.String("config_auth_source", m.config.AuthSource))

	return uri
}

func (m *MongoDB) createClient(
	ctx context.Context,
	uri string,
	clientType string,
) (*mongo.Client, error) {
	m.logger.Debug("Creating MongoDB client",
		zap.String("client_type", clientType),
		zap.String("deployment_type", m.config.Type),
		zap.String("auth_source", m.config.AuthSource))

	clientOptions := options.Client().ApplyURI(uri)

	// Cấu hình connection pool
	m.configureClientOptions(clientOptions, clientType)

	// Set read preference based on client type
	if clientType == "read" {
		switch MongoDeployment(m.config.Type) {
		case MongoReplicaSet:
			clientOptions.SetReadPreference(readpref.SecondaryPreferred())
			m.logger.Debug("Set read preference to SecondaryPreferred for replica set")
		case MongoSharded:
			clientOptions.SetReadPreference(readpref.Nearest())
			m.logger.Debug("Set read preference to Nearest for sharded cluster")
		default:
			clientOptions.SetReadPreference(readpref.Primary())
			m.logger.Debug("Set read preference to Primary for single instance")
		}
	} else {
		clientOptions.SetReadPreference(readpref.Primary())
		m.logger.Debug("Set read preference to Primary for write client")
	}

	// Connect
	m.logger.Debug("Connecting to MongoDB")
	client, err := mongo.Connect(clientOptions)
	if err != nil {
		m.logger.Error("Failed to connect to MongoDB",
			zap.String("client_type", clientType),
			zap.Error(err))
		return nil, err
	}

	// Test connection
	m.logger.Debug("Testing MongoDB connection")
	if err := client.Ping(ctx, clientOptions.ReadPreference); err != nil {
		m.logger.Error("Failed to ping MongoDB",
			zap.String("client_type", clientType),
			zap.Error(err))
		client.Disconnect(ctx)
		return nil, err
	}

	m.logger.Debug("MongoDB client created successfully",
		zap.String("client_type", clientType))
	return client, nil
}

func (m *MongoDB) configureClientOptions(options *options.ClientOptions, clientType string) {
	m.logger.Debug("Configuring MongoDB client options",
		zap.String("client_type", clientType),
		zap.Uint64("max_pool_size", m.config.MaxPoolSize),
		zap.Uint64("min_pool_size", m.config.MinPoolSize),
		zap.Int("max_conn_idle_time_seconds", m.config.MaxConnIdleTime),
		zap.Int("connect_timeout_seconds", m.config.ConnectTimeout))

	// Override với config values
	if m.config.MaxPoolSize > 0 {
		options.SetMaxPoolSize(m.config.MaxPoolSize)
	}
	if m.config.MinPoolSize > 0 {
		options.SetMinPoolSize(m.config.MinPoolSize)
	}
	if m.config.MaxConnIdleTime > 0 {
		options.SetMaxConnIdleTime(time.Duration(m.config.MaxConnIdleTime) * time.Second)
	}

	// Set retry options
	options.SetRetryReads(true)
	options.SetRetryWrites(clientType == "write")

	// Set timeout options
	options.SetConnectTimeout(time.Duration(m.config.ConnectTimeout) * time.Second)
	options.SetServerSelectionTimeout(5 * time.Second)

	m.logger.Debug("MongoDB client options configured successfully",
		zap.String("client_type", clientType))
}

func (m *MongoDB) testConnections(
	ctx context.Context,
	writeHost string,
	writePort int,
	readHost string,
	readPort int,
) error {
	m.logger.Debug("Testing MongoDB single instance connections")

	// Test write client
	m.logger.Debug("Testing write client connection")
	if err := m.writeClient.Ping(ctx, readpref.Primary()); err != nil {
		m.logger.Error("Failed to ping write client",
			zap.String("write_host", writeHost),
			zap.Int("write_port", writePort),
			zap.Error(err))
		return fmt.Errorf("failed to ping write client: %w", err)
	}
	m.logger.Debug("Write client connection test passed")

	// Test read client
	readPref := readpref.Primary()
	if m.readClient != m.writeClient {
		readPref = readpref.SecondaryPreferred()
		m.logger.Debug("Using SecondaryPreferred read preference for separate read client")
	} else {
		m.logger.Debug("Using Primary read preference for shared client")
	}

	m.logger.Debug("Testing read client connection")
	if err := m.readClient.Ping(ctx, readPref); err != nil {
		m.logger.Error("Failed to ping read client",
			zap.String("read_host", readHost),
			zap.Int("read_port", readPort),
			zap.Error(err))
		return fmt.Errorf("failed to ping read client: %w", err)
	}
	m.logger.Debug("Read client connection test passed")

	m.logger.Info("Successfully connected to MongoDB Single",
		zap.String("write_host", writeHost),
		zap.Int("write_port", writePort),
		zap.String("read_host", readHost),
		zap.Int("read_port", readPort))
	return nil
}

func (m *MongoDB) testReplicaSetConnections(ctx context.Context) error {
	m.logger.Debug("Testing MongoDB replica set connections")

	// Test write client (primary)
	m.logger.Debug("Testing primary connection")
	if err := m.writeClient.Ping(ctx, readpref.Primary()); err != nil {
		m.logger.Error("Failed to ping primary", zap.Error(err))
		return fmt.Errorf("failed to ping primary: %w", err)
	}
	m.logger.Debug("Primary connection test passed")

	// Test read client (secondary preferred)
	m.logger.Debug("Testing secondary connection")
	if err := m.readClient.Ping(ctx, readpref.SecondaryPreferred()); err != nil {
		m.logger.Error("Failed to ping secondary", zap.Error(err))
		return fmt.Errorf("failed to ping secondary: %w", err)
	}
	m.logger.Debug("Secondary connection test passed")

	m.logger.Debug("Replica set connection tests completed successfully")
	return nil
}

func (m *MongoDB) testShardedConnections(ctx context.Context) error {
	m.logger.Debug("Testing MongoDB sharded cluster connections")

	// Test write client
	m.logger.Debug("Testing sharded cluster write connection")
	if err := m.writeClient.Ping(ctx, readpref.Primary()); err != nil {
		m.logger.Error("Failed to ping sharded cluster (write)", zap.Error(err))
		return fmt.Errorf("failed to ping sharded cluster (write): %w", err)
	}
	m.logger.Debug("Sharded cluster write connection test passed")

	// Test read client
	m.logger.Debug("Testing sharded cluster read connection")
	if err := m.readClient.Ping(ctx, readpref.Nearest()); err != nil {
		m.logger.Error("Failed to ping sharded cluster (read)", zap.Error(err))
		return fmt.Errorf("failed to ping sharded cluster (read): %w", err)
	}
	m.logger.Debug("Sharded cluster read connection test passed")

	m.logger.Debug("Sharded cluster connection tests completed successfully")
	return nil
}

func (m *MongoDB) Close() error {
	m.logger.Info("Closing MongoDB connections")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var errs []error

	if m.writeClient != nil && m.writeClient != m.readClient {
		m.logger.Debug("Disconnecting write client")
		if err := m.writeClient.Disconnect(ctx); err != nil {
			m.logger.Error("Write client disconnect error", zap.Error(err))
			errs = append(errs, fmt.Errorf("write client disconnect error: %w", err))
		} else {
			m.logger.Info("MongoDB write client disconnected successfully")
		}
	}

	if m.readClient != nil {
		m.logger.Debug("Disconnecting read client")
		if err := m.readClient.Disconnect(ctx); err != nil {
			m.logger.Error("Read client disconnect error", zap.Error(err))
			errs = append(errs, fmt.Errorf("read client disconnect error: %w", err))
		} else {
			m.logger.Info("MongoDB read client disconnected successfully")
		}
	}

	if len(errs) > 0 {
		m.logger.Error(
			"MongoDB disconnect completed with errors",
			zap.Int("error_count", len(errs)),
		)
		return fmt.Errorf("disconnect errors: %v", errs)
	}

	m.logger.Info("MongoDB connections closed successfully")
	return nil
}

func (m *MongoDB) Ping() error {
	m.logger.Debug("Starting MongoDB ping")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if m.writeClient != nil {
		m.logger.Debug("Pinging write client")
		if err := m.writeClient.Ping(ctx, readpref.Primary()); err != nil {
			m.logger.Error("Write client ping failed", zap.Error(err))
			return fmt.Errorf("write client ping failed: %w", err)
		}
		m.logger.Debug("Write client ping successful")
	} else {
		m.logger.Warn("Write client is nil during ping")
	}

	if m.readClient != nil && m.readClient != m.writeClient {
		readPref := readpref.SecondaryPreferred()
		if MongoDeployment(m.config.Type) == MongoSharded {
			readPref = readpref.Nearest()
			m.logger.Debug("Using Nearest read preference for sharded cluster")
		} else {
			m.logger.Debug("Using SecondaryPreferred read preference")
		}

		m.logger.Debug("Pinging read client")
		if err := m.readClient.Ping(ctx, readPref); err != nil {
			m.logger.Error("Read client ping failed", zap.Error(err))
			return fmt.Errorf("read client ping failed: %w", err)
		}
		m.logger.Debug("Read client ping successful")
	} else if m.readClient == nil {
		m.logger.Warn("Read client is nil during ping")
	} else {
		m.logger.Debug("Read client same as write client, skipping separate ping")
	}

	m.logger.Debug("MongoDB ping completed successfully")
	return nil
}

func (m *MongoDB) GetReadConnection() interface{} {
	return m.readDB
}

func (m *MongoDB) GetWriteConnection() interface{} {
	return m.writeDB
}

func (m *MongoDB) GetType() DatabaseType {
	return MongoDBNoSQL
}

func (m *MongoDB) IsConnected() bool {
	return m.writeClient != nil && m.readClient != nil
}

func (m *MongoDB) HealthCheck() map[string]error {
	m.logger.Debug("Starting MongoDB health check")
	result := make(map[string]error)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check write client
	if m.writeClient != nil {
		m.logger.Debug("Checking write client health")
		err := m.writeClient.Ping(ctx, readpref.Primary())
		result["write_client"] = err
		if err != nil {
			m.logger.Error("Write client health check failed", zap.Error(err))
		} else {
			m.logger.Debug("Write client health check passed")
		}
	} else {
		err := fmt.Errorf("write client not initialized")
		result["write_client"] = err
		m.logger.Warn("Write client not initialized during health check")
	}

	// Check read client
	if m.readClient != nil {
		m.logger.Debug("Checking read client health")
		var readPref *readpref.ReadPref
		switch MongoDeployment(m.config.Type) {
		case MongoReplicaSet:
			readPref = readpref.SecondaryPreferred()
			m.logger.Debug("Using SecondaryPreferred for replica set health check")
		case MongoSharded:
			readPref = readpref.Nearest()
			m.logger.Debug("Using Nearest for sharded cluster health check")
		default:
			readPref = readpref.Primary()
			m.logger.Debug("Using Primary for single instance health check")
		}

		err := m.readClient.Ping(ctx, readPref)
		result["read_client"] = err
		if err != nil {
			m.logger.Error("Read client health check failed", zap.Error(err))
		} else {
			m.logger.Debug("Read client health check passed")
		}
	} else {
		err := fmt.Errorf("read client not initialized")
		result["read_client"] = err
		m.logger.Warn("Read client not initialized during health check")
	}

	// Additional deployment-specific checks
	switch MongoDeployment(m.config.Type) {
	case MongoReplicaSet:
		m.logger.Debug("Checking replica set status")
		result["replica_set_status"] = m.checkReplicaSetStatus(ctx)
	case MongoSharded:
		m.logger.Debug("Checking shard status")
		result["shard_status"] = m.checkShardStatus(ctx)
	}

	m.logger.Debug("MongoDB health check completed", zap.Any("results", result))
	return result
}

func (m *MongoDB) checkReplicaSetStatus(ctx context.Context) error {
	m.logger.Debug("Checking replica set status")

	if m.writeClient == nil {
		m.logger.Error("Write client not available for replica set status check")
		return fmt.Errorf("write client not available")
	}

	// Run replSetGetStatus command
	m.logger.Debug("Running replSetGetStatus command")
	var result bson.M
	err := m.writeClient.Database("admin").RunCommand(ctx, bson.D{
		{Key: "replSetGetStatus", Value: 1},
	}).Decode(&result)

	if err != nil {
		m.logger.Error("Failed to get replica set status", zap.Error(err))
		return fmt.Errorf("failed to get replica set status: %w", err)
	}

	// Check if we have members
	if members, ok := result["members"].(bson.A); ok && len(members) > 0 {
		m.logger.Debug("Replica set status check passed", zap.Int("member_count", len(members)))
		return nil
	}

	m.logger.Error("No replica set members found")
	return fmt.Errorf("no replica set members found")
}

func (m *MongoDB) checkShardStatus(ctx context.Context) error {
	m.logger.Debug("Checking shard status")

	if m.writeClient == nil {
		m.logger.Error("Write client not available for shard status check")
		return fmt.Errorf("write client not available")
	}

	// Run sh.status() equivalent command
	m.logger.Debug("Running listShards command")
	var result bson.M
	err := m.writeClient.Database("config").RunCommand(ctx, bson.D{
		{Key: "listShards", Value: 1},
	}).Decode(&result)

	if err != nil {
		m.logger.Error("Failed to get shard status", zap.Error(err))
		return fmt.Errorf("failed to get shard status: %w", err)
	}

	// Check if we have shards
	if shards, ok := result["shards"].(bson.A); ok && len(shards) > 0 {
		m.logger.Debug("Shard status check passed", zap.Int("shard_count", len(shards)))
		return nil
	}

	m.logger.Error("No shards found")
	return fmt.Errorf("no shards found")
}

// GetMongoStats trả về thống kê của MongoDB connections
func (m *MongoDB) GetMongoStats() map[string]interface{} {
	stats := make(map[string]interface{})

	stats["deployment_type"] = string(m.config.Type)
	stats["database"] = m.config.Database
	stats["auth_source"] = m.config.AuthSource

	if MongoDeployment(m.config.Type) == MongoReplicaSet {
		stats["replica_set_name"] = m.config.ReplicaSetName
		stats["hosts"] = m.config.Hosts
	} else if MongoDeployment(m.config.Type) == MongoSharded {
		stats["mongos_hosts"] = m.config.Hosts
	}

	return stats
}

// Helper functions để làm việc với MongoDB

// GetMongoReadDB helper function để lấy MongoDB read database
func GetMongoReadDB(db Database) (*mongo.Database, error) {
	if db.GetType() != MongoDBNoSQL {
		return nil, fmt.Errorf("database is not MongoDB")
	}
	conn, ok := db.GetReadConnection().(*mongo.Database)
	if !ok {
		return nil, fmt.Errorf("failed to cast to *mongo.Database")
	}
	return conn, nil
}

// GetMongoWriteDB helper function để lấy MongoDB write database
func GetMongoWriteDB(db Database) (*mongo.Database, error) {
	if db.GetType() != MongoDBNoSQL {
		return nil, fmt.Errorf("database is not MongoDB")
	}
	conn, ok := db.GetWriteConnection().(*mongo.Database)
	if !ok {
		return nil, fmt.Errorf("failed to cast to *mongo.Database")
	}
	return conn, nil
}

// WithMongoTransaction thực hiện transaction với write client
func WithMongoTransaction(db Database, fn func(context.Context) error) error {
	logger := zap.L()
	logger.Debug("Starting MongoDB transaction")

	mongoDB, err := GetMongoWriteDB(db)
	if err != nil {
		logger.Error("Failed to get MongoDB write database", zap.Error(err))
		return err
	}

	ctx := context.Background()
	logger.Debug("Starting MongoDB session")
	session, err := mongoDB.Client().StartSession()
	if err != nil {
		logger.Error("Failed to start MongoDB session", zap.Error(err))
		return fmt.Errorf("failed to start session: %w", err)
	}
	defer session.EndSession(ctx)
	logger.Debug("MongoDB session started successfully")

	// Use the session context for the transaction
	logger.Debug("Creating session context")
	sessionCtx := mongo.NewSessionContext(ctx, session)

	logger.Debug("Executing transaction")
	_, err = session.WithTransaction(
		sessionCtx,
		func(sc context.Context) (interface{}, error) {
			return nil, fn(sc)
		},
	)

	if err != nil {
		logger.Error("MongoDB transaction failed", zap.Error(err))
	} else {
		logger.Debug("MongoDB transaction completed successfully")
	}

	return err
}
