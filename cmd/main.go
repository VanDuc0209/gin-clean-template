package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/duccv/go-clean-template/config"
	"github.com/duccv/go-clean-template/pkg/logger"
	http_server "github.com/duccv/go-clean-template/pkg/server/http"
	"go.uber.org/zap"

	_ "github.com/duccv/go-clean-template/docs"
)

//	@title			CONTENT SERVICE APIs
//	@version		1.0
//	@description	Content service Swagger APIs.
//	@termsOfService	http://swagger.io/terms/
//	@contact.name	DucCV
//	@contact.email	duccv@gviet.vn

// @securityDefinitions.apikey	BearerAuth
// @in							header
// @name						Authorization
// @description				JWT authorization header
func main() {
	env := config.GetEnv()

	zapLogger := logger.GetLogger(env.LoggerConfig)
	zap.ReplaceGlobals(zapLogger)
	defer zapLogger.Sync()

	timeout := time.Duration(env.AppConfig.Timeout) * time.Second

	httpServer := http_server.New(env,
		http_server.Port(env.AppConfig.Port),
		http_server.Timeout(timeout),
	)

	httpServer.Start()

	// Waiting signal
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	select {
	case s := <-interrupt:
		zap.L().Info("app - Run - signal: ", zap.String("signal", s.String()))
	case err := <-httpServer.Notify():
		zap.L().Error("app - Run - httpServer.Notify: ", zap.Error(err))
	}

	// Shutdown
	if err := httpServer.Shutdown(); err != nil {
		zap.L().Error("app - Run - httpServer.Shutdown: ", zap.Error(err))
	} else {
		zap.L().Info("app - Run - httpServer shutdown gracefully")
	}
}
