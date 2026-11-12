package conf

import (
	"flag"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/rs/zerolog/log"
	xtime "github.com/Terry-Mao/goim/pkg/time"
	xlog "github.com/txchat/im-pkg/log"
	"github.com/uber/jaeger-client-go"
	traceConfig "github.com/uber/jaeger-client-go/config"
)

var (
	confPath string
	regAddrs string

	// Conf config
	Conf *Config
)

func init() {
	var (
		defAddrs = os.Getenv("REGADDRS")
	)
	flag.StringVar(&confPath, "conf", "logic.toml", "default config path")
	flag.StringVar(&regAddrs, "reg", defAddrs, "etcd register addrs. eg:127.0.0.1:2379")
}

// Init init config.
func Init() (err error) {
	Conf = Default()
	bs, err := ioutil.ReadFile(confPath)
	if err != nil {
		return err
	}
	// 与 dtalk 一致：支持 ${MYSQL_PASSWORD}/${PRESENCE_URL}/${CLUSTER_ID} 等占位
	// 去掉 CRLF，避免 presence.url 含 \r 时 toml 报 strings cannot contain newlines
	filled := strings.ReplaceAll(os.ExpandEnv(string(bs)), "\r", "")
	if _, err = toml.Decode(filled, &Conf); err != nil {
		return err
	}
	if Conf.Presence != nil {
		if v := strings.TrimSpace(os.Getenv("PRESENCE_URL")); v != "" {
			Conf.Presence.URL = strings.TrimSpace(strings.ReplaceAll(v, "\r", ""))
		} else if Conf.Presence.URL != "" {
			Conf.Presence.URL = strings.TrimSpace(strings.ReplaceAll(Conf.Presence.URL, "\r", ""))
		}
		if v := strings.TrimSpace(os.Getenv("CLUSTER_ID")); v != "" {
			Conf.Presence.ClusterID = strings.TrimSpace(strings.ReplaceAll(v, "\r", ""))
		} else if Conf.Presence.ClusterID != "" {
			Conf.Presence.ClusterID = strings.TrimSpace(strings.ReplaceAll(Conf.Presence.ClusterID, "\r", ""))
		}
	}
	return nil
}

// Default new a config with specified defualt value.
func Default() *Config {
	return &Config{
		Env: "",
		Log: xlog.Config{
			Level:   "debug",
			Mode:    "console",
			Path:    "",
			Display: "console",
		},
		Trace: traceConfig.Configuration{
			ServiceName: "answer",
			Gen128Bit:   true,
			Sampler: &traceConfig.SamplerConfig{
				Type:  jaeger.SamplerTypeConst,
				Param: 1,
			},
			Reporter: &traceConfig.ReporterConfig{
				LogSpans:           true,
				LocalAgentHostPort: "127.0.0.1:6831",
			},
		},
		Reg: &Reg{
			Schema:   "im",
			SrvName:  "logic",
			RegAddrs: regAddrs,
		},
		CometRPCClient: &RPCClient{
			Schema:  "im",
			SrvName: "comet",
			Dial:    xtime.Duration(time.Second),
			Timeout: xtime.Duration(time.Second)},
		RPCServer: &RPCServer{
			Network:           "tcp",
			Addr:              "3119",
			Timeout:           xtime.Duration(time.Second),
			IdleTimeout:       xtime.Duration(time.Second * 60),
			MaxLifeTime:       xtime.Duration(time.Hour * 2),
			ForceCloseWait:    xtime.Duration(time.Second * 20),
			KeepAliveInterval: xtime.Duration(time.Second * 60),
			KeepAliveTimeout:  xtime.Duration(time.Second * 20),
		},
		Backoff: &Backoff{MaxDelay: 300, BaseDelay: 3, Factor: 1.8, Jitter: 1.3},
		LoginLog: &LoginLogMysql{
			Driver:          "mysql",
			DSN:             "",
			MaxOpenConns:    100,
			MaxIdleConns:    10,
			ConnMaxLifetime: xtime.Duration(time.Hour),
		},
	}
}

// Config config.
type Config struct {
	Env            string
	Log            xlog.Config
	Trace          traceConfig.Configuration
	Reg            *Reg
	CometRPCClient *RPCClient
	RPCServer      *RPCServer
	Kafka          *Kafka
	Redis          *Redis
	Node           *Node
	Backoff        *Backoff
	Apps           []*App
	// LoginLog 登录审计写库；dsn 为空时不启用（见 logic.toml [loginLog]）；启用后 Startup 会通过 LogLoginLogConfig 打出脱敏快照
	LoginLog *LoginLogMysql `toml:"loginLog"`
	// Presence 中心 Presence 上报；url 或 clusterId 为空则禁用
	Presence *PresenceConfig `toml:"presence"`
}

// PresenceConfig 联邦 Presence 上报配置
type PresenceConfig struct {
	URL       string         `toml:"url"`
	ClusterID string         `toml:"clusterId"`
	Timeout   xtime.Duration `toml:"timeout"`
}

// LoginLogMysql 登录审计写库，表 dtalk_login_log 位于 dtalk_record（DSN 需包含该库名）
type LoginLogMysql struct {
	Driver          string         `toml:"driver"`
	DSN             string         `toml:"dsn"`
	MaxOpenConns    int            `toml:"maxOpenConns"`
	MaxIdleConns    int            `toml:"maxIdleConns"`
	ConnMaxLifetime xtime.Duration `toml:"connMaxLifetime"`
}

// Reg is service register/discovery config
type Reg struct {
	Schema   string
	SrvName  string // call
	RegAddrs string // etcd addrs, seperate by ','
}

type App struct {
	AppId   string
	AuthUrl string
	Timeout xtime.Duration
}

// Node node config.
type Node struct {
	HeartbeatMax int
	Heartbeat    xtime.Duration
}

// Backoff backoff.
type Backoff struct {
	MaxDelay  int32
	BaseDelay int32
	Factor    float32
	Jitter    float32
}

// Redis .
type Redis struct {
	Network      string
	Addr         string
	Auth         string
	Active       int
	Idle         int
	DialTimeout  xtime.Duration
	ReadTimeout  xtime.Duration
	WriteTimeout xtime.Duration
	IdleTimeout  xtime.Duration
	Expire       xtime.Duration
}

// Kafka .
type Kafka struct {
	Topic   string
	Brokers []string
}

// RPCClient is RPC client config.
type RPCClient struct {
	Schema  string
	SrvName string
	Dial    xtime.Duration
	Timeout xtime.Duration
}

// RPCServer is RPC server config.
type RPCServer struct {
	Network           string
	Addr              string
	Timeout           xtime.Duration
	IdleTimeout       xtime.Duration
	MaxLifeTime       xtime.Duration
	ForceCloseWait    xtime.Duration
	KeepAliveInterval xtime.Duration
	KeepAliveTimeout  xtime.Duration
}

// MaskMySQLDSNPwdForLog 将 DSN 中 user:password 的 password 替换为 ***，仅用于日志展示。
func MaskMySQLDSNPwdForLog(dsn string) string {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return ""
	}
	authEnd := strings.Index(dsn, "@tcp(")
	if authEnd == -1 {
		authEnd = strings.Index(dsn, "@unix(")
	}
	if authEnd <= 0 {
		return "***"
	}
	auth := dsn[:authEnd]
	rest := dsn[authEnd:]
	colon := strings.Index(auth, ":")
	if colon <= 0 {
		return "***" + rest
	}
	user := strings.TrimSpace(auth[:colon])
	return user + ":***" + rest
}

// LogLoginLogConfig 在进程的日志系统初始化（例如 main 里的 xlog.Init）之后调用，打印 [loginLog] 加载结果；dsn 已脱敏。
func (c *Config) LogLoginLogConfig() {
	if c == nil {
		log.Info().Msg("conf: LoginLog LogLoginLogConfig: config pointer is nil")
		return
	}
	if c.LoginLog == nil {
		log.Info().Msg("conf: [loginLog] nil (no persistence config merged)")
		return
	}
	ll := c.LoginLog
	dsnTrim := strings.TrimSpace(ll.DSN)
	log.Info().
		Str("driver", ll.Driver).
		Str("dsnMasked", MaskMySQLDSNPwdForLog(ll.DSN)).
		Int("maxOpenConns", ll.MaxOpenConns).
		Int("maxIdleConns", ll.MaxIdleConns).
		Dur("connMaxLifetime", time.Duration(ll.ConnMaxLifetime)).
		Bool("dsnConfigured", dsnTrim != "").
		Msg("conf: [loginLog] settings")
}
