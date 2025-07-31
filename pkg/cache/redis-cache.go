package cache

import (
	"context"
	"strings"
	"time"

	"github.com/duccv/go-clean-template/config"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func NewRedisClient(cfg config.RedisConfig) *redis.Client {
	var client *redis.Client
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	switch cfg.Type {
	case "NORMAL":
		client = redis.NewClient(&redis.Options{
			Addr:     cfg.Addrs,
			Password: cfg.Password,
		})
	case "SENTINEL":
		client = redis.NewFailoverClient(&redis.FailoverOptions{
			SentinelAddrs: strings.Split(cfg.Addrs, " "),
			MasterName:    cfg.MasterName,
			Password:      cfg.Password,
			ReadTimeout:   100 * time.Millisecond,
		})
	default:
		zap.S().Errorf("Invalid Redis type: %s. Must be 'normal' or 'sentinel'.", cfg.Type)
	}
	if _, err := client.Ping(ctx).Result(); err != nil {
		zap.L().Error("Failed to connect to Redis", zap.Error(err))
		return nil
	}

	zap.L().Info("Connected to Redis!!!")
	return client
}
