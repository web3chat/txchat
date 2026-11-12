package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/web3chat/txchat/dtalk/service/cs/config"
	"github.com/web3chat/txchat/dtalk/service/cs/server/http"
	"github.com/web3chat/txchat/dtalk/service/cs/service"
)

const srvName = "cs"

var log = log15.New("cmd", srvName)

func main() {
	flag.Parse()
	if err := config.Init(); err != nil {
		panic(err)
	}

	hasAES := "无(明文私钥)"
	if config.Conf.AESKey != "" {
		hasAES = "已配置"
	}
	log.Info("========== CS 服务配置 ==========",
		"Env", config.Conf.Env,
		"Addr", config.Conf.Server.Addr,
		"MySQL", fmt.Sprintf("%s@%s:%d/%s", config.Conf.MySQL.User, config.Conf.MySQL.Host, config.Conf.MySQL.Port, config.Conf.MySQL.Db),
		"AppId", config.Conf.AppId,
		"JWTSecret_len", len(config.Conf.JWTSecret),
		"AESKey", hasAES,
		"Reg", fmt.Sprintf("%s/%s@%s", config.Conf.Reg.Schema, config.Conf.Reg.SrvName, config.Conf.Reg.RegAddrs),
	)

	if config.Conf.OfflineAI == nil {
		log.Info("OfflineAI", "loaded", false)
	} else {
		log.Info("OfflineAI",
			"loaded", true,
			"Enable", config.Conf.OfflineAI.Enable,
			"CometURL_set", strings.TrimSpace(config.Conf.OfflineAI.CometURL) != "",
			"DeepSeekKey_set", strings.TrimSpace(config.Conf.OfflineAI.DeepSeekAPIKey) != "",
			"DebugIgnoreAgentStatus", config.Conf.OfflineAI.DebugIgnoreAgentStatus,
		)
	}

	// service init
	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

	svc := service.New(config.Conf)
	svc.StartOfflineAI(rootCtx)

	httpSrv := http.Init(svc)

	fmt.Printf("[%s] server started at %s\n", srvName, config.Conf.Server.Addr)
	log.Info("CS 服务已启动", "addr", config.Conf.Server.Addr)

	// init signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT)
	for {
		s := <-c
		log.Info("service get a signal", "signal", s.String())
		switch s {
		case syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT:
			rootCancel()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := httpSrv.Shutdown(ctx); err != nil {
				log.Error("server shutdown error", "err", err)
			}
			time.Sleep(time.Second * 2)
			log.Info(srvName + " server exit")
			return
		case syscall.SIGHUP:
			// TODO reload
		default:
			return
		}
	}
}
