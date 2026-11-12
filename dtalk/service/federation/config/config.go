package config

import (
	"flag"
	"io/ioutil"
	"os"
	"time"

	"github.com/BurntSushi/toml"
	xtime "github.com/web3chat/txchat/dtalk/pkg/time"
)

var (
	// confPath 默认配置路径；由 RegisterFlags 注册 -conf，避免 init 副作用导致被其他进程 import 时重复注册 flag。
	confPath = "federation.toml"
	Conf     *Config
)

// RegisterFlags 注册本服务命令行参数，须在 flag.Parse 之前由 federation 进程入口调用。
func RegisterFlags() {
	flag.StringVar(&confPath, "conf", confPath, "default config path.")
}

func Init() error {
	Conf = Default()
	bs, err := ioutil.ReadFile(confPath)
	if err != nil {
		return err
	}
	_, err = toml.Decode(os.ExpandEnv(string(bs)), &Conf)
	return err
}

func Default() *Config {
	return &Config{
		ClusterID: "cluster-a",
		Enabled:   true,
		GRPCServer: &GRPCServer{
			Addr:    "0.0.0.0:18060",
			Timeout: xtime.Duration(2 * time.Second),
		},
		MySQL: &MySQL{
			Host: "127.0.0.1", Port: "3306", User: "root", Pwd: "", DbName: "dtalk_record",
		},
		Presence: &Presence{
			URL:     "http://127.0.0.1:18050",
			Timeout: xtime.Duration(2 * time.Second),
		},
		LogicRPC: &LogicRPC{
			Schema:  "im",
			SrvName: "logic",
			RegAddrs: "127.0.0.1:2379",
			Dial:    xtime.Duration(time.Second),
			Timeout: xtime.Duration(time.Second),
		},
		Outbox: &Outbox{
			MaxRetry:      20,
			PollInterval:  xtime.Duration(2 * time.Second),
			RPCTimeout:    xtime.Duration(2 * time.Second),
			BatchSize:     50,
			LeaseDuration: xtime.Duration(30 * time.Second),
		},
		SourceCluster: "",
	}
}

type Config struct {
	ClusterID     string
	Enabled       bool
	AdminHTTPAddr string `toml:"adminHttpAddr"`
	GRPCServer    *GRPCServer
	MySQL         *MySQL
	Presence      *Presence
	LogicRPC      *LogicRPC
	Outbox        *Outbox
	SourceCluster string // 本集群作为源时的 clusterId，默认等于 ClusterID
}

type GRPCServer struct {
	Addr    string
	Timeout xtime.Duration
}

type MySQL struct {
	Host, Port, User, Pwd, DbName string
}

type Presence struct {
	URL     string
	Timeout xtime.Duration
}

type LogicRPC struct {
	Schema   string
	SrvName  string
	RegAddrs string
	Dial     xtime.Duration
	Timeout  xtime.Duration
}

type Outbox struct {
	MaxRetry      int
	PollInterval  xtime.Duration
	RPCTimeout    xtime.Duration
	BatchSize     int
	LeaseDuration xtime.Duration `toml:"leaseDuration"`
}

func (c *Config) EffectiveClusterID() string {
	if c.SourceCluster != "" {
		return c.SourceCluster
	}
	return c.ClusterID
}
