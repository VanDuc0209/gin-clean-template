package server

import (
	"fmt"
	"time"

	"github.com/duccv/go-clean-template/config"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "github.com/duccv/go-clean-template/docs"
)

// HealthCheck godoc
//
//	@Summary		Health Check
//	@Description	Returns status 200 if the service is running
//	@Tags			Health
//	@Produce		plain
//	@Success		200	{string}	string	"OK"
//	@Router			/health [get]
func StartServer(env *config.Env) {

	pathPrefix := env.AppConfig.PathPrefix
	if pathPrefix == "" {
		pathPrefix = "/api"
	}
	if env.AppConfig.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	if env.CORSConfig.Enabled {
		corsConfig := cors.Config{
			AllowOrigins:     env.CORSConfig.AllowedOrigins,
			AllowMethods:     env.CORSConfig.AllowedMethods,
			AllowHeaders:     env.CORSConfig.AllowedHeaders,
			ExposeHeaders:    env.CORSConfig.ExposedHeaders,
			AllowCredentials: env.CORSConfig.AllowCredentials,
			MaxAge:           time.Duration(env.CORSConfig.MaxAge) * time.Second,
		}

		r.Use(cors.New(corsConfig))
	}

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.AbortWithStatusJSON(200, gin.H{"status": "ok"})
	})

	// Swagger documentation
	r.GET(pathPrefix+"/swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))

	addr := fmt.Sprintf(":%d", env.AppConfig.Port)
	r.Run(addr)
}
