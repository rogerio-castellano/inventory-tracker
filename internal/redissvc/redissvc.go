package redissvc

import (
	"context"

	"github.com/redis/go-redis/v9"
)

type RedisService struct {
	rdb *redis.Client
	ctx context.Context
}

func NewRedisService(rdb *redis.Client, ctx context.Context) *RedisService {
	return &RedisService{
		rdb: rdb,
		ctx: ctx,
	}
}

func (a *RedisService) Rdb() *redis.Client {
	return a.rdb
}

func (a *RedisService) Ctx() context.Context {
	return a.ctx
}
