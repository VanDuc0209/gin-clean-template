package http_server

import (
	"net/http"
	"sync/atomic"
	"time"

	"github.com/duccv/go-clean-template/config"
	"github.com/duccv/go-clean-template/internal/constant"
	"github.com/duccv/go-clean-template/internal/middleware"
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
//	@Description	Returns 200 with {"status":"ok"} when the service is running
//	@Tags			Health
//	@Produce		json
//	@Success		200	{object}	map[string]string
//	@Router			/health [get]

// ReadyCheck godoc
//
//	@Summary		Readiness Check
//	@Description	Returns 200 with {"status":"ready"} if service is ready, otherwise 503 with {"status":"not ready"}
//	@Tags			Health
//	@Produce		json
//	@Success		200	{object}	map[string]string
//	@Failure		503	{object}	map[string]string
//	@Router			/ready [get]

type Server struct {
	App    *gin.Engine
	notify chan error

	address string
	timeout time.Duration
}

var ready atomic.Bool

// HealthCheck godoc
//
//	@Summary		Health Check
//	@Description	Returns 200 with {"status":"ok"} when the service is running
//	@Tags			Health
//	@Produce		json
//	@Success		200	{object}	map[string]string
//	@Router			/health [get]
func healthHandler(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusOK, gin.H{"status": "ok"})
}

// ReadyCheck godoc
//
//	@Summary		Readiness Check
//	@Description	Returns 200 with {"status":"ready"} if service is ready, otherwise 503 with {"status":"not ready"}
//	@Tags			Health
//	@Produce		json
//	@Success		200	{object}	map[string]string
//	@Failure		503	{object}	map[string]string
//	@Router			/ready [get]
func readyHandler(c *gin.Context) {
	if ready.Load() {
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
		return
	}
	c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready"})
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
	c.JSON(http.StatusRequestTimeout, constant.RESPONSE_TIMEOUT)
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
	r.Use(middleware.CorrelationIDMiddleware())

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

	// Health and Readiness endpoints
	r.GET("/health", healthHandler)
	r.GET("/ready", readyHandler)

	// Giả sử app cần warm-up (kết nối DB, cache,…)
	go func() {
		// TODO: init DB, cache, external service…
		time.Sleep(10 * time.Second) // ví dụ
		ready.Store(true)            // báo là đã sẵn sàng
	}()

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
