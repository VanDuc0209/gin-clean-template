package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/duccv/go-clean-template/internal/constant"
	"github.com/duccv/go-clean-template/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

func VerifyBearerToken(secret []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("authorization")
		if token == "" {
			zap.L().Warn("Authorization header is empty")
			c.AbortWithStatusJSON(http.StatusUnauthorized, constant.UNAUTHORIZED)
			return
		}

		parts := strings.Split(token, " ")
		if len(parts) != 2 || parts[0] != "Bearer" || parts[1] == "" {
			zap.L().Warn("Invalid token format", zap.Strings("parts", parts))
			c.AbortWithStatusJSON(http.StatusUnauthorized, constant.UNAUTHORIZED)
			return
		}

		payload, err := verifyToken(parts[1], secret)
		if err != nil {
			zap.L().
				Error("Token verification failed", zap.Error(err), zap.String("token", parts[1]))

			// Check if the error is due to token expiration
			if err == jwt.ErrTokenExpired {
				c.AbortWithStatusJSON(419, constant.TOKEN_EXPIRED)
				return
			}

			c.AbortWithStatusJSON(http.StatusUnauthorized, constant.UNAUTHORIZED)
			return
		}

		c.Set("jwtPayload", *payload)
		c.Next()
	}
}
func verifyToken(tokenString string, secret []byte) (*model.JWTPayload, error) {

	// Parse the token
	parsedToken, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		return secret, nil
	})

	if err != nil {
		return nil, err
	}

	// Check if token is valid
	if !parsedToken.Valid {
		return nil, jwt.ErrSignatureInvalid
	}

	// Extract claims
	claims, ok := parsedToken.Claims.(jwt.MapClaims)
	if !ok {
		return nil, jwt.ErrInvalidKeyType
	}

	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return nil, err
	}

	var payload model.JWTPayload
	err = json.Unmarshal(claimsJSON, &payload)
	if err != nil {
		return nil, err
	}

	err = validate.Struct(payload)
	if err != nil {
		return nil, fmt.Errorf("Validate Payload error: %w", err)
	}

	return &payload, nil
}
