package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/web3chat/txchat/dtalk/pkg/logger"
	"github.com/web3chat/txchat/dtalk/service/chatadmin/config"
	httpServer "github.com/web3chat/txchat/dtalk/service/chatadmin/server/http"
	"github.com/web3chat/txchat/dtalk/service/chatadmin/service"
)

const srvName = "chatadmin"

var (
	// projectVersion 项目版本
	projectVersion = "1.0.0"
	// goVersion go版本
	goVersion = ""
	// gitCommit git提交commit id
	gitCommit = ""
	// buildTime 编译时间
	buildTime = ""
	// osArch 目标主机架构
	osArch = ""

	isShowVersion = flag.Bool("v", false, "show project version")
)

// showVersion 显示项目版本信息
func showVersion(isShow bool) {
	if isShow {
		fmt.Printf("Project: %s\n", srvName)
		fmt.Printf(" Version: %s\n", projectVersion)
		fmt.Printf(" Go Version: %s\n", goVersion)
		fmt.Printf(" Git Commit: %s\n", gitCommit)
		fmt.Printf(" Built: %s\n", buildTime)
		fmt.Printf(" OS/Arch: %s\n", osArch)
		os.Exit(0)
	}
}

func main() {
	flag.Parse()
	showVersion(*isShowVersion)

	if err := config.Init(); err != nil {
		panic(err)
	}

	// log init
	log := logger.New(config.Conf.Env, srvName)
	log.Info().Interface("Config", config.Conf).
		Msg("config info")

	// service init
	svc := service.New(config.Conf)

	// http server init
	httpSrv := httpServer.Init(svc)

	// init signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT)
	for {
		s := <-c
		log.Info().Str("signal", s.String()).Msg("service get a signal")
		switch s {
		case syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			svc.Close()
			if err := httpSrv.Shutdown(ctx); err != nil {
				log.Error().Err(err).Msg("server shutdown")
			}
			log.Info().Msg(srvName + " server exit")
			return
		case syscall.SIGHUP:
			// TODO reload
		default:
			return
		}
	}
}
