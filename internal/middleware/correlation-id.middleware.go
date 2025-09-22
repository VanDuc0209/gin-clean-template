package middleware

import (
	"context"

	"github.com/duccv/go-clean-template/internal/constant"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func CorrelationIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		cid := c.GetHeader("X-Correlation-ID")
		if cid == "" {
			cid = uuid.New().String()
		}
		ctx := context.WithValue(c.Request.Context(), constant.CorrelationIDKey, cid)
		c.Request = c.Request.WithContext(ctx)
		c.Writer.Header().Set("X-Correlation-ID", cid)
		c.Next()
	}
}
