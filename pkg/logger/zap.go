package logger

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/duccv/go-clean-template/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var zapLogger *zap.Logger

// CorrelationIDKey is the key used for correlation IDs in context
const CorrelationIDKey = "correlationId"

// initLogger initializes the Zap logger with the given configuration
func initLogger(cfg config.LoggerConfig) *zap.Logger {
	level := getLogLevel(cfg.Level, cfg.Environment)

	prodEncoderCfg := zap.NewProductionEncoderConfig()
	prodEncoderCfg.TimeKey = "timestamp"
	prodEncoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	prodEncoderCfg.EncodeLevel = zapcore.CapitalLevelEncoder

	devEncoderCfg := zap.NewDevelopmentEncoderConfig()
	devEncoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	devEncoderCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder

	fileWriter := zapcore.AddSync(&lumberjack.Logger{
		Filename:   cfg.FilePath,
		MaxSize:    cfg.MaxSize,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAge,
		LocalTime:  cfg.LocalTime,
		Compress:   cfg.Compress,
	})

	// JSON encoder for file
	fileEncoder := zapcore.NewJSONEncoder(prodEncoderCfg)

	var cores []zapcore.Core

	// Always log to file
	cores = append(cores, zapcore.NewCore(fileEncoder, fileWriter, level))

	// If not production, add colorful console log
	if cfg.Environment != "production" {
		consoleEncoder := zapcore.NewConsoleEncoder(devEncoderCfg)
		consoleWriter := zapcore.AddSync(os.Stdout)
		cores = append(cores, zapcore.NewCore(consoleEncoder, consoleWriter, level))
	}

	combinedCore := zapcore.NewTee(cores...)

	return zap.New(combinedCore,
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
		zap.AddCallerSkip(1),
	)
}

// getLogLevel returns the appropriate log level based on configuration
func getLogLevel(levelStr string, env string) zap.AtomicLevel {
	if env == "production" {
		level, err := zap.ParseAtomicLevel(levelStr)
		if err != nil || level.Level() < zapcore.InfoLevel {
			fmt.Fprintf(
				os.Stderr,
				"[Logger] ⚠️  Log level '%s' not allowed in production. Fallback to INFO\n",
				levelStr,
			)
			return zap.NewAtomicLevelAt(zapcore.InfoLevel)
		}
		return level
	}

	level, err := zap.ParseAtomicLevel(levelStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[Logger] ⚠️  Invalid log level '%s', fallback to INFO\n", levelStr)
		level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	}
	return level
}

// GetLogger returns the singleton logger instance
func GetLogger(cfg config.LoggerConfig) *zap.Logger {
	if zapLogger == nil {
		zapLogger = initLogger(cfg)
	}
	return zapLogger
}

// GetLoggerFromContext returns a logger with correlation ID from context
func GetLoggerFromContext(ctx context.Context, cfg config.LoggerConfig) *zap.Logger {
	logger := GetLogger(cfg)

	if correlationID, ok := ctx.Value(CorrelationIDKey).(string); ok && correlationID != "" {
		return logger.With(zap.String("correlation_id", correlationID))
	}

	return logger
}

// WithCorrelationID adds correlation ID to the logger
func WithCorrelationID(logger *zap.Logger, correlationID string) *zap.Logger {
	if correlationID != "" {
		return logger.With(zap.String("correlation_id", correlationID))
	}
	return logger
}

// WithRequest adds HTTP request information to the logger
func WithRequest(logger *zap.Logger, req *http.Request) *zap.Logger {
	fields := []zap.Field{
		zap.String("method", req.Method),
		zap.String("path", req.URL.Path),
		zap.String("remoteAddr", req.RemoteAddr),
		zap.String("userAgent", req.UserAgent()),
	}

	if req.Referer() != "" {
		fields = append(fields, zap.String("referer", req.Referer()))
	}

	return logger.With(fields...)
}

// WithResponse adds HTTP response information to the logger
func WithResponse(logger *zap.Logger, statusCode int, responseTime time.Duration) *zap.Logger {
	return logger.With(
		zap.Int("statusCode", statusCode),
		zap.Duration("responseTime", responseTime),
	)
}

// WithError adds error information to the logger
func WithError(logger *zap.Logger, err error) *zap.Logger {
	return logger.With(zap.Error(err))
}

// WithUser adds user information to the logger
func WithUser(logger *zap.Logger, userID string) *zap.Logger {
	return logger.With(zap.String("userId", userID))
}

// WithOperation adds operation information to the logger
func WithOperation(logger *zap.Logger, operation string) *zap.Logger {
	return logger.With(zap.String("operation", operation))
}

// WithComponent adds component information to the logger
func WithComponent(logger *zap.Logger, component string) *zap.Logger {
	return logger.With(zap.String("component", component))
}

// WithDatabase adds database operation information to the logger
func WithDatabase(logger *zap.Logger, operation, table string, duration time.Duration) *zap.Logger {
	return logger.With(
		zap.String("dbOperation", operation),
		zap.String("dbTable", table),
		zap.Duration("dbDuration", duration),
	)
}

// WithCache adds cache operation information to the logger
func WithCache(
	logger *zap.Logger,
	operation, key string,
	hit bool,
	duration time.Duration,
) *zap.Logger {
	return logger.With(
		zap.String("cacheOperation", operation),
		zap.String("cacheKey", key),
		zap.Bool("cacheHit", hit),
		zap.Duration("cacheDuration", duration),
	)
}

// WithMetrics adds metrics information to the logger
func WithMetrics(
	logger *zap.Logger,
	metricName string,
	value float64,
	labels map[string]string,
) *zap.Logger {
	fields := []zap.Field{
		zap.String("metric_name", metricName),
		zap.Float64("metric_value", value),
	}

	for key, value := range labels {
		fields = append(fields, zap.String("metric_label_"+key, value))
	}

	return logger.With(fields...)
}

// Sync flushes any buffered log entries
func Sync() error {
	if zapLogger != nil {
		return zapLogger.Sync()
	}
	return nil
}
