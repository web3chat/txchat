package dao

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gomodule/redigo/redis"
	"github.com/web3chat/txchat/dtalk/service/chatclean/config"
)

// Dao holds the database connection for chatclean service.
type Dao struct {
	db       *sql.DB
	recordDb string // 消息记录库名（dtalk_msg_relation / dtalk_msg_content 所在库）
	redis    *redis.Pool
}

func New(c *config.Config) *Dao {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=Local",
		c.MySQL.User, c.MySQL.Pwd, c.MySQL.Host, c.MySQL.Port, c.MySQL.Db)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		panic(fmt.Sprintf("chatclean dao: open mysql: %v", err))
	}
	if err = db.Ping(); err != nil {
		panic(fmt.Sprintf("chatclean dao: ping mysql: %v", err))
	}
	db.SetMaxOpenConns(100)
	db.SetMaxIdleConns(10)

	recordDb, err := validIdent(c.MySQL.RecordDb, "dtalk_record")
	if err != nil {
		panic(fmt.Sprintf("chatclean dao: record db: %v", err))
	}
	return &Dao{db: db, recordDb: recordDb, redis: newOptionalRedis(c)}
}

func (d *Dao) Close() error {
	if d.redis != nil {
		_ = d.redis.Close()
	}
	return d.db.Close()
}

func (d *Dao) HasRedis() bool {
	return d.redis != nil
}

func newOptionalRedis(c *config.Config) *redis.Pool {
	if c == nil || !c.Redis.Enable || c.Redis.Addr == "" {
		return nil
	}
	addr := c.Redis.Addr
	auth := c.Redis.Auth
	return &redis.Pool{
		MaxIdle:     4,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			opts := []redis.DialOption{
				redis.DialConnectTimeout(2 * time.Second),
				redis.DialReadTimeout(3 * time.Second),
				redis.DialWriteTimeout(3 * time.Second),
			}
			if auth != "" {
				opts = append(opts, redis.DialPassword(auth))
			}
			return redis.Dial("tcp", addr, opts...)
		},
	}
}

func (d *Dao) DB() *sql.DB {
	return d.db
}
