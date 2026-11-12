package dao

import (
	"context"
	"database/sql"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql" // MySQL 驱动
	"github.com/rs/zerolog/log"

	"github.com/web3chat/txchat/im/logic/conf"
)

// LoginLogDAO 用户登录日志（与 Gateway 原 dtalk_login_log 写入一致）
type LoginLogDAO struct {
	db *sql.DB
}

// NewLoginLogDAO dsn 为空时返回 nil（不写库）
func NewLoginLogDAO(c *conf.Config) *LoginLogDAO {
	if c == nil || c.LoginLog == nil {
		log.Info().Msg("loginLog: skip init, config or [loginLog] is nil")
		return nil
	}
	cfg := c.LoginLog
	dsn := strings.TrimSpace(cfg.DSN)
	if dsn == "" {
		log.Info().Msg("loginLog: empty DSN, login log persistence disabled")
		return nil
	}
	driver := cfg.Driver
	if driver == "" {
		driver = "mysql"
	}
	dsnMasked := conf.MaskMySQLDSNPwdForLog(dsn)
	maxOpen := cfg.MaxOpenConns
	if maxOpen <= 0 {
		maxOpen = 100
	}
	maxIdle := cfg.MaxIdleConns
	if maxIdle <= 0 {
		maxIdle = 10
	}
	log.Info().
		Str("driver", driver).
		Str("dsnMasked", dsnMasked).
		Int("maxOpenConns", maxOpen).
		Int("maxIdleConns", maxIdle).
		Dur("connMaxLifetime", time.Duration(cfg.ConnMaxLifetime)).
		Msg("loginLog: connecting to MySQL (sql.Open + Ping)")

	db, err := sql.Open(driver, dsn)
	if err != nil {
		log.Error().Err(err).
			Str("driver", driver).
			Str("dsnMasked", dsnMasked).
			Msg("loginLog: sql.Open failed, database connection not established")
		panic(err)
	}
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	if time.Duration(cfg.ConnMaxLifetime) > 0 {
		db.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime))
	}
	if err := db.Ping(); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			log.Warn().Err(closeErr).Str("dsnMasked", dsnMasked).Msg("loginLog: db.Close after failed Ping")
		}
		log.Error().Err(err).
			Str("driver", driver).
			Str("dsnMasked", dsnMasked).
			Msg("loginLog: db.Ping failed, database connection not established")
		panic(err)
	}
	log.Info().
		Str("driver", driver).
		Str("dsnMasked", dsnMasked).
		Int("maxOpenConns", maxOpen).
		Int("maxIdleConns", maxIdle).
		Dur("connMaxLifetime", time.Duration(cfg.ConnMaxLifetime)).
		Msg("loginLog: MySQL connection successful (Ping OK)")
	return &LoginLogDAO{db: db}
}

// Close 关闭连接池
func (d *LoginLogDAO) Close() error {
	if d == nil || d.db == nil {
		return nil
	}
	return d.db.Close()
}

const insertLoginLog = `INSERT INTO dtalk_login_log (uid, client_ip, login_at, device_type, device_uuid, device_name, country, city_region, isp) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

// Insert 插入一条登录日志
func (d *LoginLogDAO) Insert(ctx context.Context, uid, clientIP string, loginAt int64, deviceType, deviceUUID, deviceName, country, cityRegion, isp string) error {
	if d == nil || d.db == nil {
		return nil
	}
	res, err := d.db.ExecContext(ctx, insertLoginLog, uid, clientIP, loginAt, deviceType, deviceUUID, deviceName, country, cityRegion, isp)
	if err != nil {
		log.Error().Err(err).
			Str("uid", uid).
			Str("clientIP", clientIP).
			Int64("loginAt", loginAt).
			Str("deviceType", deviceType).
			Str("deviceUUID", deviceUUID).
			Str("deviceName", deviceName).
			Str("country", country).
			Str("cityRegion", cityRegion).
			Str("isp", isp).
			Msg("loginLog: INSERT dtalk_login_log failed")
		return err
	}
	n, raErr := res.RowsAffected()
	if raErr != nil {
		log.Warn().Err(raErr).Str("uid", uid).Msg("loginLog: RowsAffected after INSERT failed")
	}
	log.Info().
		Str("uid", uid).
		Str("clientIP", clientIP).
		Int64("loginAt", loginAt).
		Str("deviceType", deviceType).
		Str("deviceUUID", deviceUUID).
		Str("deviceName", deviceName).
		Str("country", country).
		Str("cityRegion", cityRegion).
		Str("isp", isp).
		Int64("rowsAffected", n).
		Msg("loginLog: INSERT dtalk_login_log success")
	return nil
}
