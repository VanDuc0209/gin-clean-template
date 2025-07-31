package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORSMiddleware provides CORS functionality
type CORSMiddleware struct {
	config *MiddlewareConfig
}

// NewCORSMiddleware creates a new CORS middleware
func NewCORSMiddleware(config *MiddlewareConfig) *CORSMiddleware {
	return &CORSMiddleware{
		config: config,
	}
}

// CORS provides CORS middleware
func (c *CORSMiddleware) CORS() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if !c.config.CORSEnabled {
			ctx.Next()
			return
		}

		origin := ctx.GetHeader("Origin")
		if origin == "" {
			ctx.Next()
			return
		}

		// Check if origin is allowed
		if !c.isOriginAllowed(origin) {
			ctx.Next()
			return
		}

		// Set CORS headers
		ctx.Header("Access-Control-Allow-Origin", origin)
		ctx.Header("Access-Control-Allow-Credentials", "true")
		ctx.Header("Access-Control-Allow-Methods", strings.Join(c.config.CORSMethods, ", "))
		ctx.Header("Access-Control-Allow-Headers", strings.Join(c.config.CORSHeaders, ", "))
		ctx.Header("Access-Control-Max-Age", fmt.Sprintf("%.0f", c.config.CORSMaxAge.Seconds()))

		// Handle preflight requests
		if ctx.Request.Method == "OPTIONS" {
			ctx.AbortWithStatus(http.StatusNoContent)
			return
		}

		ctx.Next()
	}
}

// isOriginAllowed checks if the origin is allowed
func (c *CORSMiddleware) isOriginAllowed(origin string) bool {
	// Allow all origins if configured
	if len(c.config.CORSOrigins) == 1 && c.config.CORSOrigins[0] == "*" {
		return true
	}

	// Check against allowed origins
	for _, allowedOrigin := range c.config.CORSOrigins {
		if origin == allowedOrigin {
			return true
		}
	}

	return false
}

// StrictCORS provides strict CORS middleware for production
func (c *CORSMiddleware) StrictCORS() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if !c.config.CORSEnabled {
			ctx.Next()
			return
		}

		origin := ctx.GetHeader("Origin")
		if origin == "" {
			ctx.Next()
			return
		}

		// Only allow specific origins in strict mode
		if !c.isOriginAllowed(origin) {
			ctx.Status(http.StatusForbidden)
			ctx.Abort()
			return
		}

		// Set strict CORS headers
		ctx.Header("Access-Control-Allow-Origin", origin)
		ctx.Header("Access-Control-Allow-Credentials", "true")
		ctx.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		ctx.Header(
			"Access-Control-Allow-Headers",
			"Origin, Content-Type, Accept, Authorization, X-Requested-With",
		)
		ctx.Header("Access-Control-Max-Age", "86400") // 24 hours

		// Handle preflight requests
		if ctx.Request.Method == "OPTIONS" {
			ctx.AbortWithStatus(http.StatusNoContent)
			return
		}

		ctx.Next()
	}
}

// DevelopmentCORS provides permissive CORS for development
func (c *CORSMiddleware) DevelopmentCORS() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// Allow all origins in development
		ctx.Header("Access-Control-Allow-Origin", "*")
		ctx.Header("Access-Control-Allow-Credentials", "true")
		ctx.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
		ctx.Header("Access-Control-Allow-Headers", "*")
		ctx.Header("Access-Control-Expose-Headers", "Content-Length, Content-Type, Authorization")
		ctx.Header("Access-Control-Max-Age", "86400")

		// Handle preflight requests
		if ctx.Request.Method == "OPTIONS" {
			ctx.AbortWithStatus(http.StatusNoContent)
			return
		}

		ctx.Next()
	}
}
