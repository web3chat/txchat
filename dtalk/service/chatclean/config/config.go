package config

import (
	"flag"
	"io/ioutil"
	"os"
	"time"

	"github.com/BurntSushi/toml"
	xgrpc "github.com/web3chat/txchat/dtalk/pkg/net/grpc"
	xtime "github.com/web3chat/txchat/dtalk/pkg/time"
	xlog "github.com/txchat/im-pkg/log"
)

var (
	confPath string
	regAddrs string
	env      string

	Conf *Config
)

func init() {
	defAddrs := os.Getenv("REGADDRS")
	defEnv := os.Getenv("DTALKENV")
	flag.StringVar(&confPath, "conf", "chatclean.toml", "default config path.")
	flag.StringVar(&regAddrs, "reg", defAddrs, "etcd register addrs. eg:127.0.0.1:2379")
	flag.StringVar(&env, "env", defEnv, "service runtime environment")
}

func Init() (err error) {
	Conf = Default()
	bs, err := ioutil.ReadFile(confPath)
	if err != nil {
		return err
	}
	filled := os.ExpandEnv(string(bs))
	_, err = toml.Decode(filled, &Conf)
	return
}

func Default() *Config {
	return &Config{
		AppId: "dtalk",
		Env:   env,
		Log: xlog.Config{
			Level:   "debug",
			Mode:    xlog.ConsoleMode,
			Path:    "",
			Display: xlog.JsonDisplay,
		},
		Reg: &Reg{
			Schema:   "dtalk",
			SrvName:  "chatclean",
			RegAddrs: regAddrs,
		},
		GRPCServer: &xgrpc.ServerConfig{
			Network: "tcp",
			Addr:    ":30010",
			Timeout: xtime.Duration(time.Second),
		},
		MySQL: &MySQL{
			Host:     "127.0.0.1",
			Port:     3306,
			User:     "root",
			Pwd:      "123456",
			Db:       "dtalk",
			RecordDb: "dtalk_record",
		},
		Scheduler: &Scheduler{
			Interval:  xtime.Duration(time.Hour),
			BatchSize: 500,
		},
		Retention: Retention{
			Enable:     true,
			Keep:       xtime.Duration(168 * time.Hour),
			MaxBatches: 200,
		},
		MinIO: MinIO{
			Enable:     false,
			MaxObjects: 2000,
		},
	}
}

type Config struct {
	AppId      string
	Env        string
	Log        xlog.Config
	GRPCServer *xgrpc.ServerConfig
	Reg        *Reg
	MySQL      *MySQL
	Scheduler  *Scheduler
	Retention  Retention
	Redis      Redis
	MinIO      MinIO
}

type MySQL struct {
	Host     string
	Port     int32
	User     string
	Pwd      string
	Db       string
	RecordDb string // 消息记录库名（dtalk_msg_relation / dtalk_msg_content 所在库），默认 "dtalk_record"
}

type Reg struct {
	Schema       string
	SrvName      string
	RegAddrs     string
	RegisterAddr string // 可选：显式指定注册到 etcd 的地址（如宿主机 IP:端口），为空则自动探测容器 IP
}

type Scheduler struct {
	Interval  xtime.Duration
	BatchSize int
}

// Retention 全局保留。私聊和群聊都删 create_time 早于 now-Keep 的消息。
// 不读取 dtalk_auto_delete_config。Keep 为 0 时按 168h。
type Retention struct {
	Enable     bool
	Keep       xtime.Duration
	MaxBatches int
}

// Redis 可选。配了才清理 record:{uid} 里 CreateTime 早于保留窗口的缓存。
type Redis struct {
	Enable bool
	Addr   string
	Auth   string
}

// MinIO 可选。Enable 为 false 或 Endpoint 为空时跳过。
// AccessKey / SecretKey 只放在部署配置里，文档和示例用占位符。
type MinIO struct {
	Enable     bool
	Endpoint   string
	Bucket     string
	AccessKey  string
	SecretKey  string
	UseSSL     bool
	MaxObjects int
}
