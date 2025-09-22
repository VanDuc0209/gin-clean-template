package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

func init() {
	validate = validator.New()
}

type MiddlewareConfig struct {
	// JWT Configuration
	JWTSecret []byte
	JWTExpiry time.Duration

	// Rate Limiting
	RateLimitEnabled bool
	RateLimitWindow  time.Duration
	RateLimitMax     int

	// CORS Configuration
	CORSEnabled bool
	CORSOrigins []string
	CORSMethods []string
	CORSHeaders []string
	CORSMaxAge  time.Duration

	// Logging Configuration
	LoggingEnabled  bool
	LogRequestID    bool
	LogUserAgent    bool
	LogIPAddress    bool
	LogResponseTime bool

	// Analytics Configuration
	AnalyticsEnabled bool
	TrackUserAgent   bool
	TrackIPAddress   bool
	TrackReferrer    bool
}

func DefaultMiddlewareConfig() *MiddlewareConfig {
	return &MiddlewareConfig{
		JWTExpiry:        24 * time.Hour,
		RateLimitEnabled: true,
		RateLimitWindow:  time.Minute,
		RateLimitMax:     100,
		CORSEnabled:      true,
		CORSOrigins:      []string{"*"},
		CORSMethods:      []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		CORSHeaders:      []string{"Origin", "Content-Type", "Accept", "Authorization"},
		CORSMaxAge:       12 * time.Hour,
		LoggingEnabled:   true,
		LogRequestID:     true,
		LogUserAgent:     true,
		LogIPAddress:     true,
		LogResponseTime:  true,
		AnalyticsEnabled: true,
		TrackUserAgent:   true,
		TrackIPAddress:   true,
		TrackReferrer:    true,
	}
}

// getClientIP extracts the real client IP address
func getClientIP(c *gin.Context) string {
	// Check for forwarded headers
	if ip := c.GetHeader("X-Forwarded-For"); ip != "" {
		return ip
	}
	if ip := c.GetHeader("X-Real-IP"); ip != "" {
		return ip
	}
	if ip := c.GetHeader("X-Client-IP"); ip != "" {
		return ip
	}

	return c.ClientIP()
}
