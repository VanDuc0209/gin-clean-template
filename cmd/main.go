package main

import (
	"github.com/duccv/go-clean-template/config"
	"github.com/duccv/go-clean-template/pkg/logger"
	"github.com/duccv/go-clean-template/pkg/server"
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

	server.StartServer(env)
}
