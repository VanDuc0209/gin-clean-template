package config

import (
	"fmt"
	"log"
	"strings"

	"github.com/spf13/viper"
)

type (
	AppConfig struct {
		Name        string `mapstructure:"name"`
		Version     string `mapstructure:"version"`
		Port        int    `mapstructure:"port"`
		Environment string `mapstructure:"environment"`
		PathPrefix  string `mapstructure:"path_prefix"` // Optional, can be used to set a base path for the application
	}

	LoggerConfig struct {
		Level       string `mapstructure:"level"`
		Format      string `mapstructure:"format"`
		FilePath    string `mapstructure:"filepath"`
		MaxSize     int    `mapstructure:"max_size"`
		MaxAge      int    `mapstructure:"max_age"`
		MaxBackups  int    `mapstructure:"max_backups"`
		Compress    bool   `mapstructure:"compress"`
		LocalTime   bool   `mapstructure:"localTime"`
		Environment string
	}

	PostgresConfig struct {
		Host              string `mapstructure:"host"`
		Port              int    `mapstructure:"port"`
		User              string `mapstructure:"user"`
		Password          string `mapstructure:"password"`
		Database          string `mapstructure:"database"`
		SSLMode           string `mapstructure:"sslmode"`
		ConnectionString  string `mapstructure:"connection_string"`
		ConnectionTimeout int    `mapstructure:"connection_timeout"`
		MaxIdleConns      int    `mapstructure:"max_idle_conns"`
		MaxOpenConns      int    `mapstructure:"max_open_conns"`
		ConnMaxLifetime   int    `mapstructure:"conn_max_lifetime"`
		ConnMaxIdleTime   int    `mapstructure:"conn_max_idle_time"`
	}

	MongoConfig struct {
		URI            string `mapstructure:"uri"`
		Database       string `mapstructure:"database"`
		ReplicaSet     string `mapstructure:"replicaSet"`
		AuthSource     string `mapstructure:"authSource"`
		Username       string `mapstructure:"username"`
		Password       string `mapstructure:"password"`
		ConnectTimeout int    `mapstructure:"connect_timeout"`
		MaxPoolSize    int    `mapstructure:"max_pool_size"`
		MinPoolSize    int    `mapstructure:"min_pool_size"`
		SocketTimeout  int    `mapstructure:"socket_timeout"`
	}

	CORSConfig struct {
		Enabled          bool     `mapstructure:"enabled"`
		AllowedOrigins   []string `mapstructure:"allowed_origins"`
		AllowedMethods   []string `mapstructure:"allowed_methods"`
		AllowedHeaders   []string `mapstructure:"allowed_headers"`
		ExposedHeaders   []string `mapstructure:"exposed_headers"`
		AllowCredentials bool     `mapstructure:"allow_credentials"`
		MaxAge           int      `mapstructure:"max_age"`
	}
)

type Env struct {
	AppConfig      AppConfig      `mapstructure:"app"`
	LoggerConfig   LoggerConfig   `mapstructure:"logging"`
	PostgresConfig PostgresConfig `mapstructure:"postgres"`
	MongoConfig    MongoConfig    `mapstructure:"mongo"`
	CORSConfig     CORSConfig     `mapstructure:"cors"`
}

var env Env
var envLoaded bool

func loadEnv() Env {
	// Set up viper to read the config.yaml file
	viper.SetConfigName("config")   // Config file name without extension
	viper.SetConfigType("yaml")     // Config file type
	viper.AddConfigPath("./config") // Look for the config file in the current directory

	/*
	   AutomaticEnv will check for an environment variable any time a viper.Get request is made.
	   It will apply the following rules.
	       It will check for an environment variable with a name matching the key uppercased and prefixed with the EnvPrefix if set.
	*/
	viper.AutomaticEnv()
	viper.SetEnvPrefix("env") // will be uppercased automatically
	viper.SetEnvKeyReplacer(
		strings.NewReplacer(".", "_"),
	) // this is useful e.g. want to use . in Get() calls, but environmental variables to use _ delimiters (e.g. app.port -> APP_PORT)

	err := viper.ReadInConfig() // Read the config file
	if err != nil {
		log.Fatalf("Error reading config file, %s", err)
	}

	// Set up environment variable mappings if necessary
	/*
	   BindEnv takes one or more parameters. The first parameter is the key name, the rest are the name of the environment variables to bind to this key.
	   If more than one are provided, they will take precedence in the specified order. The name of the environment variable is case sensitive.
	   If the ENV variable name is not provided, then Viper will automatically assume that the ENV variable matches the following format: prefix + "_" + the key name in ALL CAPS.
	   When you explicitly provide the ENV variable name (the second parameter), it does not automatically add the prefix.
	       For example if the second parameter is "id", Viper will look for the ENV variable "ID".
	*/
	viper.BindEnv(
		"app.name",
		"APP_NAME",
	) // Bind the app.name key to the APP_NAME environment variable

	err = viper.Unmarshal(&env)
	if err != nil {
		log.Fatalf("Unable to decode into struct, %v", err)
	}
	env.LoggerConfig.Environment = env.AppConfig.Environment // Set the logger environment from app config
	if env.AppConfig.Environment == "production" {
		env.LoggerConfig.Level = "info" // Default to info level in production
	}

	printStartupConfig(&env)

	return env
}

func GetEnv() *Env {
	if envLoaded {
		return &env
	}
	env = loadEnv()
	envLoaded = true
	return &env
}

func printStartupConfig(env *Env) {
	line := strings.Repeat("=", 40)
	fmt.Println(line)
	fmt.Println("🚀 Application Configuration")
	fmt.Println(line)

	fmt.Printf("%-15s: %s\n", "App Name", env.AppConfig.Name)
	fmt.Printf("%-15s: %s\n", "Version", env.AppConfig.Version)
	fmt.Printf("%-15s: %s\n", "Environment", env.AppConfig.Environment)
	fmt.Printf("%-15s: %d\n", "Port", env.AppConfig.Port)
	fmt.Printf("%-15s: %s\n", "Log Level", env.LoggerConfig.Level)

	fmt.Println(line)
}
