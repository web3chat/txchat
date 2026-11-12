package dao

import (
	"fmt"

	"github.com/inconshreveable/log15"
	"github.com/web3chat/txchat/dtalk/pkg/mysql"
	"github.com/web3chat/txchat/dtalk/service/cs/config"
)

type Dao struct {
	log  log15.Logger
	conn *mysql.MysqlConn
}

func New(c *config.Config) *Dao {
	d := &Dao{
		log:  log15.New("module", "cs/dao"),
		conn: newDB(c.MySQL),
	}
	return d
}

func newDB(cfg *config.MySQL) *mysql.MysqlConn {
	c, err := mysql.NewMysqlConn(cfg.Host, fmt.Sprintf("%v", cfg.Port),
		cfg.User, cfg.Pwd, cfg.Db, "UTF8MB4")
	if err != nil {
		log15.Error("mysql init failed", "err", err)
		panic(err)
	}
	return c
}
