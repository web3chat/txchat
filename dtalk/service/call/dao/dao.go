package dao

import (
	"github.com/rs/zerolog"
	"github.com/web3chat/txchat/dtalk/pkg/logger"
	"github.com/web3chat/txchat/dtalk/pkg/redis"
	"github.com/web3chat/txchat/dtalk/service/call/config"
)

var srvName = "call/dao"

type Dao struct {
	log   zerolog.Logger
	redis *redis.Pool
}

func New(cfg *config.Config) *Dao {
	d := &Dao{
		log:   logger.New(cfg.Env, srvName),
		redis: redis.New(cfg.Redis),
	}

	return d
}
