package http_server

import (
	"net/http"
	"time"

	"github.com/duccv/go-clean-template/config"
	"github.com/duccv/go-clean-template/pkg/metrics"
	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/timeout"
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

type Server struct {
	App    *gin.Engine
	notify chan error

	address string
	timeout time.Duration
}

// New -.
func New(env *config.Env, opts ...Option) *Server {
	s := &Server{
		App:     nil,
		notify:  make(chan error, 1),
		address: _defaultAddr,
		timeout: _defaultTimeout,
	}

	for _, opt := range opts {
		opt(s)
	}

	s.App = s.initGinServer(env)

	return s
}

func timeoutResponse(c *gin.Context) {
	c.String(http.StatusRequestTimeout, "timeout")
}
func timeoutMiddleware(to time.Duration) gin.HandlerFunc {
	return timeout.New(
		timeout.WithTimeout(to),
		timeout.WithResponse(timeoutResponse),
	)
}

func (s *Server) initGinServer(env *config.Env) *gin.Engine {

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
	r.Use(timeoutMiddleware(s.timeout))

	if env.MetricsConfig.Enabled {
		m := metrics.GetMonitor("/metrics")
		m.Use(r)
	}

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
	return r
}

// StartServer -.
func (s *Server) Start() {
	go func() {
		s.notify <- s.App.Run(s.address)
		close(s.notify)
	}()
}

// Notify -.
func (s *Server) Notify() <-chan error {
	return s.notify
}

// Shutdown -.
func (s *Server) Shutdown() error {
	return nil
}
