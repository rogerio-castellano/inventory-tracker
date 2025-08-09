package auth

import (
	"context"

	"github.com/redis/go-redis/v9"
)

type AuthService struct {
	rdb *redis.Client
	ctx context.Context
}

func NewAuthService(rdb *redis.Client, ctx context.Context) *AuthService {
	return &AuthService{
		rdb: rdb,
		ctx: ctx,
	}
}

func (a *AuthService) Rdb() *redis.Client {
	return a.rdb
}

func (a *AuthService) Ctx() context.Context {
	return a.ctx
}
