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
	}

	LoggerConfig struct {
		Level      string `mapstructure:"level"`
		Format     string `mapstructure:"format"`
		FilePath   string `mapstructure:"filepath"`
		MaxSize    int    `mapstructure:"max_size"`
		MaxAge     int    `mapstructure:"max_age"`
		MaxBackups int    `mapstructure:"max_backups"`
		Compress   bool   `mapstructure:"compress"`
		LocalTime  bool   `mapstructure:"localTime"`
	}
)

type Env struct {
	AppConfig    AppConfig    `mapstructure:"app"`
	LoggerConfig LoggerConfig `mapstructure:"logging"`
}

var env Env

func loadEnv() *Env {
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

	printStartupConfig(&env)

	return &env
}

func GetEnv() *Env {
	if env != (Env{}) {
		return &env
	}
	return loadEnv()
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
