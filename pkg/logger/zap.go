package logger

import (
	"fmt"
	"os"

	"github.com/duccv/go-clean-template/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var zapLogger *zap.Logger

func initLogger(cfg config.LoggerConfig) *zap.Logger {
	level := getLogLevel(cfg.Level, cfg.Environment)

	prodEncoderCfg := zap.NewProductionEncoderConfig()
	prodEncoderCfg.TimeKey = "timestamp"
	prodEncoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	devEncoderCfg := zap.NewDevelopmentEncoderConfig()
	devEncoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	devEncoderCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder

	fileWriter := zapcore.AddSync(&lumberjack.Logger{
		Filename:   cfg.FilePath,
		MaxSize:    cfg.MaxSize,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAge,
		LocalTime:  cfg.LocalTime,
		Compress:   cfg.Compress,
	})

	// JSON encoder for file
	fileEncoder := zapcore.NewJSONEncoder(prodEncoderCfg)

	var cores []zapcore.Core

	// Always log to file
	cores = append(cores, zapcore.NewCore(fileEncoder, fileWriter, level))

	// If not production, add colorful console log
	if cfg.Environment != "production" {
		consoleEncoder := zapcore.NewConsoleEncoder(devEncoderCfg)
		consoleWriter := zapcore.AddSync(os.Stdout)
		cores = append(cores, zapcore.NewCore(consoleEncoder, consoleWriter, level))
	}

	combinedCore := zapcore.NewTee(cores...)

	return zap.New(combinedCore,
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)
}

func getLogLevel(levelStr string, env string) zap.AtomicLevel {
	if env == "production" {
		level, err := zap.ParseAtomicLevel(levelStr)
		if err != nil || level.Level() < zapcore.InfoLevel {
			fmt.Fprintf(
				os.Stderr,
				"[Logger] ⚠️  Log level '%s' not allowed in production. Fallback to INFO\n",
				levelStr,
			)
			return zap.NewAtomicLevelAt(zapcore.InfoLevel)
		}
		return level
	}

	level, err := zap.ParseAtomicLevel(levelStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[Logger] ⚠️  Invalid log level '%s', fallback to INFO\n", levelStr)
		level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	}
	return level
}

func GetLogger(cfg config.LoggerConfig) *zap.Logger {
	if zapLogger == nil {
		zapLogger = initLogger(cfg)
	}
	return zapLogger
}
