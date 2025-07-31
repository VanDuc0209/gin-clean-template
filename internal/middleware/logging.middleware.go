package middleware

import (
	"bytes"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// LoggingMiddleware provides request logging functionality
type LoggingMiddleware struct {
	config *MiddlewareConfig
}

// NewLoggingMiddleware creates a new logging middleware
func NewLoggingMiddleware(config *MiddlewareConfig) *LoggingMiddleware {
	return &LoggingMiddleware{
		config: config,
	}
}

// RequestLogger provides request logging middleware
func (l *LoggingMiddleware) RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !l.config.LoggingEnabled {
			c.Next()
			return
		}

		start := time.Now()

		// Generate request ID
		requestID := generateRequestID()
		c.Set("requestId", requestID)

		// Create logger with request context
		logger := l.createRequestLogger(c, requestID)

		// Log request start
		logger.Info("Request started",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("query", c.Request.URL.RawQuery),
			zap.String("userAgent", c.GetHeader("User-Agent")),
			zap.String("referer", c.GetHeader("Referer")),
			zap.String("ip", getClientIP(c)))

		// Capture response body
		responseWriter := &responseBodyWriter{
			ResponseWriter: c.Writer,
			body:           &bytes.Buffer{},
		}
		c.Writer = responseWriter

		// Process request
		c.Next()

		// Calculate response time
		duration := time.Since(start)

		// Log request completion
		logger.Info("Request completed",
			zap.Int("status", c.Writer.Status()),
			zap.Int("size", c.Writer.Size()),
			zap.Duration("duration", duration),
			zap.String("error", c.Errors.String()))

		// Log slow requests
		if duration > 5*time.Second {
			logger.Warn("Slow request detected",
				zap.Duration("duration", duration),
				zap.String("path", c.Request.URL.Path))
		}
	}
}

// createRequestLogger creates a logger with request context
func (l *LoggingMiddleware) createRequestLogger(c *gin.Context, requestID string) *zap.Logger {
	fields := []zap.Field{
		zap.String("requestId", requestID),
		zap.String("method", c.Request.Method),
		zap.String("path", c.Request.URL.Path),
	}

	if l.config.LogIPAddress {
		fields = append(fields, zap.String("ip", getClientIP(c)))
	}

	if l.config.LogUserAgent {
		fields = append(fields, zap.String("userAgent", c.GetHeader("User-Agent")))
	}

	// Add user context if available
	if userID, exists := c.Get("userId"); exists {
		if id, ok := userID.(uint); ok {
			fields = append(fields, zap.Uint("userId", id))
		}
	}

	return zap.L().With(fields...)
}

// responseBodyWriter captures response body for logging
type responseBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *responseBodyWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *responseBodyWriter) WriteString(s string) (int, error) {
	w.body.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}

// ErrorLogger provides error logging middleware
func (l *LoggingMiddleware) ErrorLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Log errors if any
		if len(c.Errors) > 0 {
			requestID, _ := c.Get("requestId")
			logger := zap.L().With(zap.String("requestId", requestID.(string)))

			for _, err := range c.Errors {
				logger.Error("Request error",
					zap.String("error", err.Error()),
					zap.String("path", c.Request.URL.Path),
					zap.String("method", c.Request.Method))
			}
		}
	}
}

// SecurityLogger provides security event logging
func (l *LoggingMiddleware) SecurityLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Log security-relevant events
		requestID, _ := c.Get("requestId")
		logger := zap.L().With(zap.String("requestId", requestID.(string)))

		// Log authentication attempts
		if c.Request.Method == "POST" &&
			(c.Request.URL.Path == "/api/v1/auth/login" || c.Request.URL.Path == "/api/v1/auth/register") {
			logger.Info("Authentication attempt",
				zap.String("path", c.Request.URL.Path),
				zap.String("ip", getClientIP(c)),
				zap.String("userAgent", c.GetHeader("User-Agent")))
		}

		// Log failed authentication
		if c.Writer.Status() == http.StatusUnauthorized {
			logger.Warn("Authentication failed",
				zap.String("path", c.Request.URL.Path),
				zap.String("ip", getClientIP(c)),
				zap.String("userAgent", c.GetHeader("User-Agent")))
		}

		c.Next()
	}
}

// PerformanceLogger provides performance monitoring
func (l *LoggingMiddleware) PerformanceLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		duration := time.Since(start)

		// Log performance metrics
		if l.config.LogResponseTime {
			requestID, _ := c.Get("requestId")
			logger := zap.L().With(zap.String("requestId", requestID.(string)))

			logger.Info("Performance metrics",
				zap.Duration("responseTime", duration),
				zap.String("path", c.Request.URL.Path),
				zap.String("method", c.Request.Method),
				zap.Int("status", c.Writer.Status()),
				zap.Int("size", c.Writer.Size()))
		}
	}
}

// RequestIDMiddleware adds request ID to all requests
func (l *LoggingMiddleware) RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if request ID is already set
		if requestID := c.GetHeader("X-Request-ID"); requestID != "" {
			c.Set("requestId", requestID)
		} else {
			// Generate new request ID
			requestID := generateRequestID()
			c.Set("requestId", requestID)
			c.Header("X-Request-ID", requestID)
		}

		c.Next()
	}
}
