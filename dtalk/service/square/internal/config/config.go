package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env           string
	AllowMock     bool
	HTTP          HTTPConfig
	Postgres      PostgresConfig
	JWT           JWTConfig
	DeepSeek      DeepSeekConfig
	OSS           OSSConfig
	Bootstrap     BootstrapConfig
	CORS          []string
	SchemaPath    string
	AdminWeb      string
	HotWindowDays int
	Retention     RetentionConfig
}

type RetentionConfig struct {
	TTLDays  int
	Interval time.Duration
}

type HTTPConfig struct {
	Addr string
}

type PostgresConfig struct {
	Host    string
	Port    string
	User    string
	Pwd     string
	DBName  string
	SSLMode string
	// Auto 为 true 表示未指定 SQUARE_PG_HOST，本地可自动拉起 Docker PostgreSQL。
	Auto bool
}

type JWTConfig struct {
	Secret string
	Expire time.Duration
}

type DeepSeekConfig struct {
	APIKey      string
	BaseURL     string
	Model       string
	VisionModel string
	Timeout     time.Duration
}

type BootstrapAccount struct {
	User string
	Pass string
	Role string
}

type BootstrapConfig struct {
	AdminUser string
	AdminPass string
	// ResetAdminPass 为 true 时，启动用 AdminPass 覆盖根管理员已有密码。用完应改回 false。
	ResetAdminPass bool
	// Accounts 来自 SQUARE_BOOTSTRAP_ADMINS：用户名:密码:角色，多条用逗号分隔。
	Accounts []BootstrapAccount
}

// OSSConfig 对接谈信 IM nginx 上的 OSS（与 ChatPro 同一入口）。
type OSSConfig struct {
	BaseURL       string
	AppID         string
	PrivKey       string
	DeleteEnabled bool
}

func (c OSSConfig) UploadURL() string {
	base := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	if base == "" {
		return ""
	}
	if strings.HasSuffix(base, "/oss/upload") {
		return base
	}
	if strings.HasSuffix(base, "/oss") {
		return base + "/upload"
	}
	return base + "/oss/upload"
}

func (c OSSConfig) DeleteURL() string {
	base := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	if base == "" {
		return ""
	}
	if strings.HasSuffix(base, "/oss/delete") {
		return base
	}
	if strings.HasSuffix(base, "/oss/upload") {
		return strings.TrimSuffix(base, "/upload") + "/delete"
	}
	if strings.HasSuffix(base, "/oss") {
		return base + "/delete"
	}
	return base + "/oss/delete"
}

var Conf *Config

func Init() error {
	cfg := &Config{
		Env:       envOr("SQUARE_ENV", "local"),
		AllowMock: envBool("SQUARE_ALLOW_MOCK", true),
		HTTP: HTTPConfig{
			Addr: envOr("SQUARE_HTTP_ADDR", ":18100"),
		},
		Postgres: loadPostgres(),
		JWT: JWTConfig{
			Secret: envOr("SQUARE_JWT_SECRET", "dev-square-jwt-secret-change-me-32b"),
			Expire: 8 * time.Hour,
		},
		DeepSeek: DeepSeekConfig{
			APIKey:      os.Getenv("DEEPSEEK_API_KEY"),
			BaseURL:     envOr("DEEPSEEK_BASE_URL", "https://api.deepseek.com"),
			Model:       envOr("DEEPSEEK_MODEL", "deepseek-flash"),
			VisionModel: envOr("DEEPSEEK_VISION_MODEL", "deepseek-flash"),
			Timeout:     8 * time.Second,
		},
		OSS: OSSConfig{
			BaseURL:       envOr("SQUARE_OSS_URL", ""),
			AppID:         envOr("SQUARE_OSS_APP_ID", "dtalk"),
			PrivKey:       envOr("SQUARE_OSS_PRIVKEY", ""),
			DeleteEnabled: envBool("SQUARE_OSS_DELETE", false),
		},
		Bootstrap: BootstrapConfig{
			AdminUser:      strings.TrimSpace(os.Getenv("SQUARE_BOOTSTRAP_ADMIN_USER")),
			AdminPass:      os.Getenv("SQUARE_BOOTSTRAP_ADMIN_PASSWORD"),
			ResetAdminPass: envBool("SQUARE_ADMIN_PASSWORD_RESET", false),
			Accounts:       parseBootstrapAdmins(os.Getenv("SQUARE_BOOTSTRAP_ADMINS")),
		},
		CORS:          splitCSV(envOr("SQUARE_CORS", "http://127.0.0.1:18100,http://localhost:18100,http://127.0.0.1:18101,http://localhost:18101")),
		SchemaPath:    envOr("SQUARE_SCHEMA_PATH", "sql/schema.sql"),
		AdminWeb:      envOr("SQUARE_ADMIN_WEB", "web/admin"),
		HotWindowDays: clampInt(envInt("SQUARE_HOT_WINDOW_DAYS", 7), 1, 30, 7),
		Retention: RetentionConfig{
			TTLDays:  clampTTLDays(envInt("SQUARE_POST_TTL_DAYS", 45)),
			Interval: envDuration("SQUARE_RETENTION_INTERVAL", time.Hour),
		},
	}
	if IsProd(cfg.Env) {
		cfg.AllowMock = false
		if len(cfg.JWT.Secret) < 32 {
			return fmt.Errorf("SQUARE_JWT_SECRET must be at least 32 chars in prod")
		}
		if cfg.Postgres.Auto {
			return fmt.Errorf("SQUARE_PG_HOST is required in prod")
		}
	}
	Conf = cfg
	return nil
}

func IsProd(env string) bool {
	e := strings.ToLower(strings.TrimSpace(env))
	return e == "prod" || e == "production"
}

func IsMockAllowed(env string, allow bool) bool {
	if IsProd(env) {
		return false
	}
	e := strings.ToLower(strings.TrimSpace(env))
	if e != "local" && e != "dev" && e != "test" {
		return false
	}
	return allow
}

func loadPostgres() PostgresConfig {
	host := strings.TrimSpace(os.Getenv("SQUARE_PG_HOST"))
	port := strings.TrimSpace(os.Getenv("SQUARE_PG_PORT"))
	auto := host == ""
	if auto {
		host = "127.0.0.1"
		if port == "" {
			port = "15432"
		}
	} else if port == "" {
		port = "5432"
	}
	return PostgresConfig{
		Host:    host,
		Port:    port,
		User:    envOr("SQUARE_PG_USER", "square"),
		Pwd:     envOr("SQUARE_PG_PASSWORD", "square"),
		DBName:  envOr("SQUARE_PG_DB", "square"),
		SSLMode: envOr("SQUARE_PG_SSLMODE", "disable"),
		Auto:    auto,
	}
}

func envOr(key, def string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	return v
}

func envBool(key string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func envInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func envDuration(key string, def time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return def
	}
	return d
}

func clampInt(v, lo, hi, def int) int {
	if v < lo || v > hi {
		return def
	}
	return v
}

// clampTTLDays：0 关闭；非 0 钳到 30–62 天。
func clampTTLDays(v int) int {
	if v <= 0 {
		return 0
	}
	if v < 30 {
		return 30
	}
	if v > 62 {
		return 62
	}
	return v
}

// parseBootstrapAdmins 解析「用户名:密码:角色」，多条用逗号分隔。
// 密码至少在启动时再校验长度；这里只丢掉字段不全的条目。
func parseBootstrapAdmins(raw string) []BootstrapAccount {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]BootstrapAccount, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		fields := strings.SplitN(part, ":", 3)
		if len(fields) != 3 {
			continue
		}
		user := strings.TrimSpace(fields[0])
		if user == "" {
			continue
		}
		out = append(out, BootstrapAccount{
			User: user,
			Pass: fields[1],
			Role: strings.TrimSpace(fields[2]),
		})
	}
	return out
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
