package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/sirupsen/logrus"

	"github.com/web3chat/txchat/dtalk/service/shardkey/config"
)

// InitDB 建立 MySQL 连接。
//
// 表结构统一由 script/mysql/shardkey.sql 管理，服务启动时不再执行 DDL，
// 避免多实例同时 ALTER、以及新库漏字段（is_deleted）的问题。
func InitDB(cfg *config.Config) (*sql.DB, error) {
	db, err := sql.Open("mysql", cfg.MySQL.DSN())
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	db.SetMaxOpenConns(50)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(time.Hour)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping mysql: %w", err)
	}
	if err := checkSchema(db); err != nil {
		return nil, err
	}
	return db, nil
}

func checkSchema(db *sql.DB) error {
	checks := []struct {
		table string
		query string
	}{
		{"shard_keys", `SELECT id, phone, chataddress, shard_key, previous_shard_key, previous_chataddress, is_updated, version, is_deleted, created_at, updated_at FROM shard_keys LIMIT 0`},
		{"user_bindings", `SELECT id, phone, wechat_id, xiaohongshu_id, douyin_id, chataddress, is_deleted, created_at, updated_at FROM user_bindings LIMIT 0`},
	}
	for _, c := range checks {
		rows, err := db.Query(c.query)
		if err != nil {
			return fmt.Errorf("table %s is not ready, run script/mysql/shardkey.sql first: %w", c.table, err)
		}
		_ = rows.Close()
	}
	logrus.Info("shardkey schema check passed")
	return nil
}
