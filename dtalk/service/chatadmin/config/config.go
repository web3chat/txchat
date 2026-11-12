package config

import (
	"flag"
	"io/ioutil"
	"os"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/web3chat/txchat/dtalk/pkg/redis"
	xtime "github.com/web3chat/txchat/dtalk/pkg/time"
)

var (
	confPath string
	Conf     *Config
)

func init() {
	flag.StringVar(&confPath, "conf", "chatadmin.toml", "default config path.")
}

// Init init config.
func Init() (err error) {
	Conf = Default()
	bs, err := ioutil.ReadFile(confPath)
	if err != nil {
		return err
	}
	filled := os.ExpandEnv(string(bs))
	_, err = toml.Decode(filled, &Conf)
	return err
}

func Default() *Config {
	return &Config{
		Env: "debug",
		HttpServer: &HttpServer{
			Addr: "0.0.0.0:18020",
		},
		MySQL: &MySQL{
			Host: "127.0.0.1",
			Port: 3306,
			User: "root",
			Pwd:  "123456",
			Db:   "dtalk",
		},
		MySQLRecord: &MySQL{
			Host: "127.0.0.1",
			Port: 3306,
			User: "root",
			Pwd:  "123456",
			Db:   "dtalk_record",
		},
		// MySQLAuthKey 默认 nil：user_bindings 走 MySQL（dtalk）；正式环境拆库时在配置里增加 [MySQLAuthKey]
		MySQLAuthKey: nil,
		Redis: &redis.Config{
			Network:      "tcp",
			Addr:         "127.0.0.1:6379",
			Auth:         "",
			Active:       60000,
			Idle:         1024,
			DialTimeout:  xtime.Duration(200 * time.Millisecond),
			ReadTimeout:  xtime.Duration(500 * time.Millisecond),
			WriteTimeout: xtime.Duration(500 * time.Millisecond),
			IdleTimeout:  xtime.Duration(120 * time.Second),
			Expire:       xtime.Duration(30 * time.Minute),
		},
		Auth: &Auth{
			JwtSecret:     "dtalk-chatadmin-secret",
			JwtTTLSeconds: 7200,
		},
		Cors: &Cors{
			AllowOrigins: []string{
				"http://localhost:8000",
				"http://127.0.0.1:8000",
				"http://localhost:8000",
			},
			AllowCredentials: true,
		},
	}
}

type Config struct {
	Env         string
	HttpServer  *HttpServer
	MySQL       *MySQL
	MySQLRecord *MySQL // dtalk_record 数据库连接配置
	// MySQLAuthKey 可选：存放 user_bindings 的独立库（如 authkey）；nil 时表示仍在 MySQL.Db（dtalk）中
	MySQLAuthKey *MySQL
	Redis        *redis.Config
	Auth         *Auth
	Cors         *Cors
}

type HttpServer struct {
	Addr string
}

// Cors 浏览器跨域配置（manage-web 直连 chatadmin 时必须配置前端 Origin）
type Cors struct {
	AllowOrigins     []string
	AllowCredentials bool
}

type MySQL struct {
	Host string
	Port int32
	User string
	Pwd  string
	Db   string
}

type Auth struct {
	JwtSecret     string
	JwtTTLSeconds int64
}
