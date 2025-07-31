package metrics

import (
	"context"
	"time"

	"github.com/penglongli/gin-metrics/ginmetrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

var (
	// HTTP Request Metrics
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status_code"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"method", "path"},
	)

	// Cache Metrics
	cacheHitsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_hits_total",
			Help: "Total number of cache hits",
		},
		[]string{"cache_type", "operation"},
	)

	cacheMissesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_misses_total",
			Help: "Total number of cache misses",
		},
		[]string{"cache_type", "operation"},
	)

	cacheOperationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "cache_operation_duration_seconds",
			Help:    "Cache operation duration in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5},
		},
		[]string{"cache_type", "operation"},
	)

	// Database Metrics
	dbConnectionsActive = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "db_connections_active",
			Help: "Number of active database connections",
		},
		[]string{"database", "type"},
	)

	dbConnectionsIdle = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "db_connections_idle",
			Help: "Number of idle database connections",
		},
		[]string{"database", "type"},
	)

	dbConnectionsMax = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "db_connections_max",
			Help: "Maximum number of database connections",
		},
		[]string{"database", "type"},
	)

	dbOperationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "db_operation_duration_seconds",
			Help:    "Database operation duration in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5},
		},
		[]string{"database", "operation", "table"},
	)

	dbErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_errors_total",
			Help: "Total number of database errors",
		},
		[]string{"database", "operation", "error_type"},
	)

	// Redis Metrics
	redisConnectionsActive = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "redis_connections_active",
			Help: "Number of active Redis connections",
		},
		[]string{"redis_instance"},
	)

	redisOperationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "redis_operation_duration_seconds",
			Help:    "Redis operation duration in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5},
		},
		[]string{"redis_instance", "operation"},
	)

	redisErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "redis_errors_total",
			Help: "Total number of Redis errors",
		},
		[]string{"redis_instance", "operation", "error_type"},
	)
)

// MetricsCollector provides methods to collect various metrics
type MetricsCollector struct {
	logger *zap.Logger
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(logger *zap.Logger) *MetricsCollector {
	return &MetricsCollector{
		logger: logger,
	}
}

// RecordHTTPRequest records HTTP request metrics
func (mc *MetricsCollector) RecordHTTPRequest(
	method, path string,
	statusCode int,
	duration time.Duration,
) {
	labels := prometheus.Labels{
		"method":      method,
		"path":        path,
		"status_code": string(rune(statusCode)),
	}

	httpRequestsTotal.With(labels).Inc()
	httpRequestDuration.With(prometheus.Labels{
		"method": method,
		"path":   path,
	}).Observe(duration.Seconds())

	mc.logger.Debug("Recorded HTTP request metrics",
		zap.String("method", method),
		zap.String("path", path),
		zap.Int("status_code", statusCode),
		zap.Duration("duration", duration),
	)
}

// RecordCacheOperation records cache operation metrics
func (mc *MetricsCollector) RecordCacheOperation(
	cacheType, operation string,
	hit bool,
	duration time.Duration,
) {
	labels := prometheus.Labels{
		"cache_type": cacheType,
		"operation":  operation,
	}

	if hit {
		cacheHitsTotal.With(labels).Inc()
	} else {
		cacheMissesTotal.With(labels).Inc()
	}

	cacheOperationDuration.With(labels).Observe(duration.Seconds())

	mc.logger.Debug("Recorded cache operation metrics",
		zap.String("cache_type", cacheType),
		zap.String("operation", operation),
		zap.Bool("hit", hit),
		zap.Duration("duration", duration),
	)
}

// RecordDatabaseOperation records database operation metrics
func (mc *MetricsCollector) RecordDatabaseOperation(
	database, operation, table string,
	duration time.Duration,
	err error,
) {
	labels := prometheus.Labels{
		"database":  database,
		"operation": operation,
		"table":     table,
	}

	dbOperationDuration.With(labels).Observe(duration.Seconds())

	if err != nil {
		errorLabels := prometheus.Labels{
			"database":   database,
			"operation":  operation,
			"error_type": "database_error",
		}
		dbErrorsTotal.With(errorLabels).Inc()
	}

	mc.logger.Debug("Recorded database operation metrics",
		zap.String("database", database),
		zap.String("operation", operation),
		zap.String("table", table),
		zap.Duration("duration", duration),
		zap.Error(err),
	)
}

// RecordDatabaseConnections records database connection pool metrics
func (mc *MetricsCollector) RecordDatabaseConnections(database string, active, idle, max int) {
	labels := prometheus.Labels{
		"database": database,
		"type":     "postgres",
	}

	dbConnectionsActive.With(labels).Set(float64(active))
	dbConnectionsIdle.With(labels).Set(float64(idle))
	dbConnectionsMax.With(labels).Set(float64(max))

	mc.logger.Debug("Recorded database connection metrics",
		zap.String("database", database),
		zap.Int("active", active),
		zap.Int("idle", idle),
		zap.Int("max", max),
	)
}

// RecordRedisOperation records Redis operation metrics
func (mc *MetricsCollector) RecordRedisOperation(
	instance, operation string,
	duration time.Duration,
	err error,
) {
	labels := prometheus.Labels{
		"redis_instance": instance,
		"operation":      operation,
	}

	redisOperationDuration.With(labels).Observe(duration.Seconds())

	if err != nil {
		errorLabels := prometheus.Labels{
			"redis_instance": instance,
			"operation":      operation,
			"error_type":     "redis_error",
		}
		redisErrorsTotal.With(errorLabels).Inc()
	}

	mc.logger.Debug("Recorded Redis operation metrics",
		zap.String("instance", instance),
		zap.String("operation", operation),
		zap.Duration("duration", duration),
		zap.Error(err),
	)
}

// RecordRedisConnections records Redis connection metrics
func (mc *MetricsCollector) RecordRedisConnections(instance string, active int) {
	labels := prometheus.Labels{
		"redis_instance": instance,
	}

	redisConnectionsActive.With(labels).Set(float64(active))

	mc.logger.Debug("Recorded Redis connection metrics",
		zap.String("instance", instance),
		zap.Int("active", active),
	)
}

func GetMonitor(path string) *ginmetrics.Monitor {
	m := ginmetrics.GetMonitor()
	// +optional set path
	m.SetMetricPath(path)
	// +optional set slow time
	m.SetSlowTime(1)

	// +optional set request duration, default {0.1, 0.3, 1.2, 5, 10}
	// used to p95, p99

	m.SetDuration([]float64{0.05, 0.1, 0.2, 0.3, 0.5, 1, 2, 5})

	// customize metrics
	// gaugeMetric := &ginmetrics.Metric{
	// 	Type:        ginmetrics.Gauge,
	// 	Name:        "example_gauge_metric",
	// 	Description: "an example of gauge type metric",
	// 	Labels:      []string{"label1"},
	// }

	// m.AddMetric(gaugeMetric)

	return m
}

// StartMetricsCollection starts background metrics collection
func StartMetricsCollection(
	ctx context.Context,
	collector *MetricsCollector,
	interval time.Duration,
) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	collector.logger.Info("Started metrics collection", zap.Duration("interval", interval))

	for {
		select {
		case <-ctx.Done():
			collector.logger.Info("Stopped metrics collection")
			return
		case <-ticker.C:
			// Collect periodic metrics here
			// This could include database connection pool stats, cache stats, etc.
		}
	}
}
