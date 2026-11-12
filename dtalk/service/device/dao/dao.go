package dao

import (
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"
	"github.com/web3chat/txchat/dtalk/pkg/mysql"
	"github.com/web3chat/txchat/dtalk/service/device/config"
)

type Dao struct {
	log   zerolog.Logger
	redis *redis.Pool
	mysql *mysql.MysqlConn
}

func New(c *config.Config) *Dao {
	mysqlConn, err := mysql.NewMysqlConn(c.MySQL.Host, c.MySQL.Port, c.MySQL.User, c.MySQL.Pwd, c.MySQL.DbName, "utf8mb4")
	if err != nil {
		panic("device dao: failed to connect mysql: " + err.Error())
	}
	d := &Dao{
		log:   zlog.Logger,
		redis: newRedis(c.Redis),
		mysql: mysqlConn,
	}
	return d
}

func newRedis(c *config.Redis) *redis.Pool {
	return &redis.Pool{
		MaxIdle:     c.Idle,
		MaxActive:   c.Active,
		IdleTimeout: time.Duration(c.IdleTimeout),
		Dial: func() (redis.Conn, error) {
			conn, err := redis.Dial(c.Network, c.Addr,
				redis.DialConnectTimeout(time.Duration(c.DialTimeout)),
				redis.DialReadTimeout(time.Duration(c.ReadTimeout)),
				redis.DialWriteTimeout(time.Duration(c.WriteTimeout)),
				redis.DialPassword(c.Auth),
			)
			if err != nil {
				return nil, err
			}
			return conn, nil
		},
	}
}
