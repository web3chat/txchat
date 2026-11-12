package config

import (
	"flag"
	"os"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	xtime "github.com/web3chat/txchat/dtalk/pkg/time"
	"github.com/uber/jaeger-client-go"
	traceConfig "github.com/uber/jaeger-client-go/config"
)

var (
	confPath string

	Conf *Config
)

func init() {
	flag.StringVar(&confPath, "conf", "gateway.toml", "default config path.")
}

// Init init config.
func Init() (err error) {
	Conf = Default()
	bs, err := os.ReadFile(confPath)
	if err != nil {
		return err
	}
	// 支持 ${MYSQL_PASSWORD}；去掉 CRLF，避免 toml 报 strings cannot contain newlines
	filled := strings.ReplaceAll(os.ExpandEnv(string(bs)), "\r", "")
	if _, err = toml.Decode(filled, &Conf); err != nil {
		return err
	}
	if v := os.Getenv("MYSQL_PASSWORD"); v != "" && Conf.Federation != nil {
		Conf.Federation.MySQLPwd = v
	}
	return nil
}

func Default() *Config {
	return &Config{
		Env: "debug",
		Server: &HttpServer{
			Addr: "0.0.0.0:18002",
		},
		Trace: traceConfig.Configuration{
			ServiceName: "gateway",
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
		Revoke: &Revoke{
			Expire: xtime.Duration(time.Hour * 24),
		},
		AnswerRPCClient: &RPCClient{
			RegAddrs: "127.0.0.1:2379",
			Schema:   "dtalk",
			SrvName:  "answer",
			Dial:     xtime.Duration(time.Second),
			Timeout:  xtime.Duration(time.Second),
		},
		StoreRPCClient: &RPCClient{
			RegAddrs: "127.0.0.1:2379",
			Schema:   "dtalk",
			SrvName:  "store",
			Dial:     xtime.Duration(time.Second),
			Timeout:  xtime.Duration(time.Second),
		},
		GroupRPCClient: &RPCClient{
			RegAddrs: "127.0.0.1:2379",
			Schema:   "dtalk",
			SrvName:  "group",
			Dial:     xtime.Duration(time.Second),
			Timeout:  xtime.Duration(time.Second),
		},
		DeviceRPCClient: &RPCClient{
			RegAddrs: "127.0.0.1:2379",
			Schema:   "dtalk",
			SrvName:  "device",
			Dial:     xtime.Duration(time.Second),
			Timeout:  xtime.Duration(time.Second),
		},
		ChatCleanRPCClient: &RPCClient{
			RegAddrs: "127.0.0.1:2379",
			Schema:   "dtalk",
			SrvName:  "chatclean",
			Dial:     xtime.Duration(time.Second),
			Timeout:  xtime.Duration(time.Second),
		},
		CSServiceURL: "http://127.0.0.1:18030",
		// 数据库配置（登录日志等）
		Database: &DatabaseConfig{
			Driver:          "mysql",
			DSN:             "root:password@tcp(localhost:3306)/dtalk?charset=utf8mb4&parseTime=True&loc=Local",
			MaxOpenConns:    100,
			MaxIdleConns:    10,
			ConnMaxLifetime: xtime.Duration(time.Hour),
		},
	}
}

type Config struct {
	Env                string
	Server             *HttpServer
	Trace              traceConfig.Configuration
	Modules            []Module
	Revoke             *Revoke
	Federation         *Federation `toml:"federation"`
	AnswerRPCClient    *RPCClient
	StoreRPCClient     *RPCClient
	GroupRPCClient     *RPCClient
	DeviceRPCClient    *RPCClient
	ChatCleanRPCClient *RPCClient
	CSServiceURL       string
	// 数据库配置（登录日志等）
	Database *DatabaseConfig
}

type HttpServer struct {
	Addr string
}

// RPCClient is RPC client config.
type RPCClient struct {
	RegAddrs string // etcd addrs, seperate by ','
	Schema   string
	SrvName  string // call
	Dial     xtime.Duration
	Timeout  xtime.Duration
}

type Module struct {
	Name      string   `json:"name"` // enums: wallet、oa、redpacket
	IsEnabled bool     `json:"isEnabled"`
	EndPoints []string `json:"endPoints"`
}

type Revoke struct {
	Expire xtime.Duration
}

// 新增：数据库配置结构
type DatabaseConfig struct {
	Driver          string         `toml:"driver"`          // 数据库驱动：mysql, postgres, sqlite
	DSN             string         `toml:"dsn"`             // 数据源名称
	MaxOpenConns    int            `toml:"maxOpenConns"`    // 最大打开连接数
	MaxIdleConns    int            `toml:"maxIdleConns"`    // 最大空闲连接数
	ConnMaxLifetime xtime.Duration `toml:"connMaxLifetime"` // 连接最大生存时间
}


// Federation 跨集群撤回等状态同步（可选）
type Federation struct {
	Enabled         bool   `toml:"enabled"`
	ClusterID       string `toml:"clusterId"`
	PresenceURL     string `toml:"presenceUrl"`
	MySQLHost       string `toml:"mysqlHost"`
	MySQLPort       string `toml:"mysqlPort"`
	MySQLUser       string `toml:"mysqlUser"`
	MySQLPwd        string `toml:"mysqlPwd"`
	MySQLDB         string `toml:"mysqlDb"`
	RPCTimeout      string `toml:"rpcTimeout"`
	PresenceTimeout string `toml:"presenceTimeout"`
}
