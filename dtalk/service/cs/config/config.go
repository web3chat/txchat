package config

import (
	"flag"
	"time"

	"github.com/BurntSushi/toml"
	xtime "github.com/web3chat/txchat/dtalk/pkg/time"
)

var (
	confPath   string
	dotenvPath string
	Conf       *Config
)

func init() {
	flag.StringVar(&confPath, "conf", "cs.toml", "default config path.")
	flag.StringVar(&dotenvPath, "dotenv", ".env", "optional .env path; empty string disables loading")
}

func Init() (err error) {
	Conf = Default()
	if _, err = toml.DecodeFile(confPath, &Conf); err != nil {
		return err
	}
	if Conf.Chain == nil {
		Conf.Chain = Default().Chain
	}
	if err := LoadDotEnv(dotenvPath); err != nil {
		return err
	}
	ApplyOfflineAIEnvOverrides(Conf)
	if Conf.OfflineAI != nil {
		if err := ExpandOfflineAITerminologyFiles(Conf.OfflineAI); err != nil {
			return err
		}
	}
	return nil
}

func Default() *Config {
	return &Config{
		Env: "debug",
		Chain: &Chain{
			RPC:               "",
			NoBalancePrivateKey: "",
			SignFee:           200000,
		},
		Server: &HttpServer{
			Addr: "0.0.0.0:18030",
		},
		Reg: &Reg{
			Schema:   "dtalk",
			SrvName:  "cs",
			RegAddrs: "127.0.0.1:2379",
		},
		MySQL: &MySQL{
			Host: "127.0.0.1",
			Port: 3306,
			User: "root",
			Pwd:  "root",
			Db:   "dtalk",
		},
		LogicRPCClient: &RPCClient{
			RegAddrs: "127.0.0.1:2379",
			Schema:   "im",
			SrvName:  "logic",
			Dial:     xtime.Duration(time.Second),
			Timeout:  xtime.Duration(time.Second),
		},
		JWTSecret: "cs-default-secret-change-me",
		AppId:     "dtalk",
	}
}

type Config struct {
	Env            string
	Server         *HttpServer
	MySQL          *MySQL
	Reg            *Reg
	LogicRPCClient *RPCClient
	JWTSecret      string
	AppId          string
	AESKey         string // 如果建账号时用了 -aes-key，这里填相同的密钥 (64位hex)；未加密则留空
	// Chain 链上绑定 IM 服务器（与 ChatPro-Android 合约节点 + 代扣私钥一致）
	Chain *Chain
	// CometReceiveAckDelayMillis 收到 Comet ReceiveMsg 后延迟再发 ReceiveMsgReply（毫秒），让 Record Pusher 先写完
	// Redis conn-seq 映射，避免对端 App 长期显示「未读」。0 或未配置=内置默认 280。cs-web 从登录/agent/info 拉取同值。
	CometReceiveAckDelayMillis int `toml:"CometReceiveAckDelayMillis"`
	// OfflineAI 无人值守 AI（DeepSeek）：仅在数据库 cs_agent.status=离线时对本 Agent 维持 Comet 连接自动应答。
	OfflineAI *OfflineAIConfig `toml:"OfflineAI"`
}

// Chain 区块链 JSON-RPC（通常 :8901）及代扣配置。
type Chain struct {
	RPC                 string `toml:"RPC"`                 // 如 http://localhost:8901
	NoBalancePrivateKey string `toml:"NoBalancePrivateKey"` // 与 App chat-pro.properties NO_BALANCE_PRIVATE_KEY 一致
	SignFee             int64  `toml:"SignFee"`             // SignRawTx 费率，默认 200000
}

type HttpServer struct {
	Addr string
}

type MySQL struct {
	Host string
	Port int32
	User string
	Pwd  string
	Db   string
}

type Reg struct {
	Schema   string
	SrvName  string
	RegAddrs string
}

type RPCClient struct {
	RegAddrs string
	Schema   string
	SrvName  string
	Dial     xtime.Duration
	Timeout  xtime.Duration
}

// OfflineAIConfig 无人值守 AI：客服在 DB 中为「离线」且 cs-web 未连接 Comet 时，由 CS 进程代为维持连接并调用 DeepSeek。
// 客服通过 cs-web 登录并将 status 置为在线后，本 Worker 会在下一轮 Poll 中退出，避免与浏览器端的同一 Agent 双开冲突。
type OfflineAIConfig struct {
	Enable               bool
	CometURL             string `toml:"CometURL"` // 与前端一致，如 http://host:8888 或 ws://host:8888，会自动补 /sub
	DeepSeekAPIKey       string `toml:"DeepSeekAPIKey"`
	DeepSeekBaseURL      string `toml:"DeepSeekBaseURL"` // 默认 https://api.deepseek.com
	Model                string // 默认 deepseek-chat
	PollInterval         xtime.Duration
	DebounceMillis       int `toml:"DebounceMillis"`       // 默认 650
	IntroDelayMillis     int `toml:"IntroDelayMillis"`     // 首轮说明后发模型前的间隔，默认 150
	HeartbeatSec         int `toml:"HeartbeatSec"`         // 默认 28
	ReconnectBackoffMax  xtime.Duration                   // 默认 30s，连接失败后睡眠上限
	DeviceUUID           string `toml:"DeviceUUID"`          // 可空，空则每个 Worker 进程内随机生成并固定
	DeviceName           string `toml:"DeviceName"`          // 默认 CS-Offline-AI
	// 话术与模型角色（均可选；空则使用代码内置默认）。
	BaseSystemPrompt string `toml:"BaseSystemPrompt"` // 角色与安全边界说明，写入 DeepSeek system 段
	Playbook         string // 业务规范，追加进 system
	HandoffClosingTemplate string
	ProactiveIntroMarkdown string // 首轮主动发给用户；空则用内置默认
	// 以下为话术外部文件路径（UTF-8）；设置则启动时读文件并覆盖对应内联字段。
	BaseSystemPromptFile string `toml:"BaseSystemPromptFile"`
	PlaybookFile         string `toml:"PlaybookFile"`
	ProactiveIntroFile   string `toml:"ProactiveIntroFile"` // 覆盖 ProactiveIntroMarkdown
	HandoffClosingFile   string `toml:"HandoffClosingFile"` // 覆盖 HandoffClosingTemplate
	// DebugIgnoreAgentStatus 为 true 时不按 DB 的在线状态过滤（仅联调）；生产务必 false。
	DebugIgnoreAgentStatus bool `toml:"DebugIgnoreAgentStatus"`
}
