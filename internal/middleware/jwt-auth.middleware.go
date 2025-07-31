package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/duccv/go-clean-template/internal/model"
	"github.com/duccv/go-clean-template/internal/model/response"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

// JWTAuthMiddleware provides JWT authentication middleware
type JWTAuthMiddleware struct {
	secret []byte
	config *MiddlewareConfig
}

// NewJWTAuthMiddleware creates a new JWT authentication middleware
func NewJWTAuthMiddleware(secret []byte, config *MiddlewareConfig) *JWTAuthMiddleware {
	return &JWTAuthMiddleware{
		secret: secret,
		config: config,
	}
}

// Authenticate validates JWT tokens and sets user context
func (m *JWTAuthMiddleware) Authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip authentication for certain endpoints
		if shouldSkipAuth(c.Request.URL.Path) {
			c.Next()
			return
		}

		token := extractToken(c)
		if token == "" {
			handleAuthError(
				c,
				http.StatusUnauthorized,
				"missing_token",
				"Authorization token required",
			)
			return
		}

		payload, err := m.verifyToken(token)
		if err != nil {
			handleAuthError(c, http.StatusUnauthorized, "invalid_token", "Invalid or expired token")
			return
		}

		// Set user context
		c.Set("userId", payload.UserID)
		c.Set("userEmail", payload.Email)
		c.Set("jwtPayload", payload)

		// Log successful authentication
		zap.L().Debug("User authenticated successfully",
			zap.Uint("userId", payload.UserID),
			zap.String("email", payload.Email),
			zap.String("path", c.Request.URL.Path))

		c.Next()
	}
}

// OptionalAuth provides optional JWT authentication (doesn't fail if no token)
func (m *JWTAuthMiddleware) OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractToken(c)
		if token == "" {
			// No token provided, continue without authentication
			c.Next()
			return
		}

		payload, err := m.verifyToken(token)
		if err != nil {
			// Token is invalid, but don't fail the request
			zap.L().Warn("Invalid optional token", zap.Error(err))
			c.Next()
			return
		}

		// Set user context
		c.Set("userId", payload.UserID)
		c.Set("userEmail", payload.Email)
		c.Set("jwtPayload", payload)

		c.Next()
	}
}

// extractToken extracts the JWT token from the Authorization header
func extractToken(c *gin.Context) string {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return ""
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return ""
	}

	return parts[1]
}

// verifyToken validates and parses the JWT token
func (m *JWTAuthMiddleware) verifyToken(tokenString string) (*model.JWTPayload, error) {
	// Parse the token
	parsedToken, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return m.secret, nil
	})

	if err != nil {
		return nil, fmt.Errorf("token parsing failed: %w", err)
	}

	// Check if token is valid
	if !parsedToken.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	// Extract claims
	claims, ok := parsedToken.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Convert claims to payload
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return nil, fmt.Errorf("claims marshaling failed: %w", err)
	}

	var payload model.JWTPayload
	if err := json.Unmarshal(claimsJSON, &payload); err != nil {
		return nil, fmt.Errorf("payload unmarshaling failed: %w", err)
	}

	// Validate payload structure
	if err := validate.Struct(payload); err != nil {
		return nil, fmt.Errorf("payload validation failed: %w", err)
	}

	// Check token expiration
	if payload.ExpiresAt != nil && time.Now().After(*payload.ExpiresAt) {
		return nil, jwt.ErrTokenExpired
	}

	return &payload, nil
}

// shouldSkipAuth determines if authentication should be skipped for the given path
func shouldSkipAuth(path string) bool {
	skipPaths := []string{
		"/health",
		"/metrics",
		"/docs",
		"/swagger",
		"/api/v1/health",
		"/api/v1/metrics",
	}

	for _, skipPath := range skipPaths {
		if strings.HasPrefix(path, skipPath) {
			return true
		}
	}

	return false
}

// handleAuthError handles authentication errors with proper logging
func handleAuthError(c *gin.Context, statusCode int, errorType, message string) {
	zap.L().Warn("Authentication failed",
		zap.String("path", c.Request.URL.Path),
		zap.String("method", c.Request.Method),
		zap.String("ip", getClientIP(c)),
		zap.String("errorType", errorType),
		zap.String("message", message))

	errorResponse := response.ResponseData{
		Ec:  statusCode,
		Msg: message,
		Error: &response.ErrorResponse{
			Type:    errorType,
			Message: message,
			Code:    statusCode,
		},
	}

	c.JSON(statusCode, errorResponse)
	c.Abort()
}

// GenerateToken generates a new JWT token for a user
func (m *JWTAuthMiddleware) GenerateToken(userID uint, email string) (string, error) {
	now := time.Now()
	expiresAt := now.Add(m.config.JWTExpiry)

	claims := jwt.MapClaims{
		"userId": userID,
		"email":  email,
		"iat":    now.Unix(),
		"exp":    expiresAt.Unix(),
		"iss":    "short-link-service",
		"aud":    "short-link-users",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(m.secret)
	if err != nil {
		return "", fmt.Errorf("token signing failed: %w", err)
	}

	return tokenString, nil
}

// RefreshToken refreshes an existing JWT token
func (m *JWTAuthMiddleware) RefreshToken(tokenString string) (string, error) {
	// Verify the existing token
	payload, err := m.verifyToken(tokenString)
	if err != nil {
		return "", fmt.Errorf("invalid token for refresh: %w", err)
	}

	// Generate a new token
	return m.GenerateToken(payload.UserID, payload.Email)
}
