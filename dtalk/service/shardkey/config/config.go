package config

import (
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/web3chat/txchat/dtalk/pkg/redis"
	xtime "github.com/web3chat/txchat/dtalk/pkg/time"
)

// Config shardkey 服务配置。
//
// 密钥类配置一律通过 ${ENV} 占位符注入，仓库内的 shardkey.toml 不保存真实值。
type Config struct {
	Env        string
	HttpServer *HttpServer
	MySQL      *MySQL
	Redis      *redis.Config
	SMS        *SMS
	Code       *Code
	Security   *Security
	// APIKeys 客户端标识 -> 固定令牌；为空时拒绝所有业务请求（fail closed）
	APIKeys map[string]string
}

type HttpServer struct {
	Addr string
}

type MySQL struct {
	Host     string
	Port     int
	User     string
	Pwd      string
	Db       string
	Charset  string
	Timezone string
}

type SMS struct {
	// AppID/AppKey/Signature 从环境变量注入，禁止写入仓库
	AppID     string
	AppKey    string
	Signature string
	// Template 验证码短信模板，%s 为验证码，%s 为用途
	Template string
}

type Code struct {
	Length        int
	Expire        xtime.Duration
	RetryInterval xtime.Duration
	MaxAttempts   int
	LockDuration  xtime.Duration
}

type Security struct {
	// EncryptKey 用于 AES-256-GCM 加密落库的分片密钥，必须由环境变量注入
	EncryptKey string
}

func Default() *Config {
	return &Config{
		Env: "release",
		HttpServer: &HttpServer{
			Addr: "0.0.0.0:18006",
		},
		MySQL: &MySQL{
			Host:     "txchat-mysql",
			Port:     3306,
			User:     "root",
			Pwd:      "${MYSQL_PASSWORD}",
			Db:       "dtalk",
			Charset:  "utf8mb4",
			Timezone: "Asia/Shanghai",
		},
		Redis: &redis.Config{
			Network:      "tcp",
			Addr:         "txchat-redis:6379",
			Auth:         "",
			Active:       60000,
			Idle:         1024,
			DialTimeout:  xtime.Duration(200 * time.Millisecond),
			ReadTimeout:  xtime.Duration(500 * time.Millisecond),
			WriteTimeout: xtime.Duration(500 * time.Millisecond),
			IdleTimeout:  xtime.Duration(120 * time.Second),
			Expire:       xtime.Duration(30 * time.Minute),
		},
		SMS: &SMS{
			Template: "您的验证码是：%s，用于%s，请在5分钟内使用。如非本人操作，请忽略。",
		},
		Code: &Code{
			Length:        6,
			Expire:        xtime.Duration(5 * time.Minute),
			RetryInterval: xtime.Duration(60 * time.Second),
			MaxAttempts:   5,
			LockDuration:  xtime.Duration(10 * time.Minute),
		},
		Security: &Security{},
		APIKeys:  map[string]string{},
	}
}

// Load 读取 TOML 配置并展开环境变量。confPath 为空时使用默认值（本地调试）。
func Load(confPath string) (*Config, error) {
	cfg := Default()
	if confPath != "" {
		bs, err := os.ReadFile(confPath)
		if err != nil {
			return nil, fmt.Errorf("read config %s: %w", confPath, err)
		}
		if _, err := toml.Decode(os.ExpandEnv(string(bs)), cfg); err != nil {
			return nil, fmt.Errorf("decode config %s: %w", confPath, err)
		}
	}
	// 允许部署环境覆盖，便于本地联调时临时开 debug（mock 短信）
	if env := os.Getenv("SHARDKEY_ENV"); env != "" {
		cfg.Env = env
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if c.HttpServer == nil || c.HttpServer.Addr == "" {
		return fmt.Errorf("http server addr is required")
	}
	if c.MySQL == nil || c.MySQL.Host == "" || c.MySQL.Db == "" {
		return fmt.Errorf("mysql host/db is required")
	}
	if c.Security == nil || c.Security.EncryptKey == "" {
		return fmt.Errorf("security.EncryptKey is required (inject via SHARDKEY_ENCRYPT_KEY)")
	}
	if c.Env == "" {
		c.Env = "release"
	}
	if c.Code == nil || c.Code.Length < 4 || c.Code.Length > 8 {
		return fmt.Errorf("code length must be between 4 and 8")
	}
	if c.Code.Expire <= 0 {
		return fmt.Errorf("code expire must be positive")
	}
	// release 必须配置真实短信；debug 允许 mock 短信（只打日志，不真正发送）
	if c.Env == "release" {
		if c.SMS == nil || c.SMS.AppID == "" || c.SMS.AppKey == "" {
			return fmt.Errorf("sms appid/appkey is required in release (inject via SUBMAIL_*)")
		}
	}
	return nil
}

// DSN 构造 MySQL DSN。
func (m *MySQL) DSN() string {
	timezone := m.Timezone
	if timezone == "" {
		timezone = "Asia/Shanghai"
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=true&loc=%s",
		m.User, m.Pwd, m.Host, m.Port, m.Db, m.Charset, url.QueryEscape(timezone))
}
