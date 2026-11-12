package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/web3chat/txchat/dtalk/service/federation/config"
	xgrpc "github.com/web3chat/txchat/dtalk/service/federation/server/grpc"
	xhttp "github.com/web3chat/txchat/dtalk/service/federation/server/http"
	"github.com/web3chat/txchat/dtalk/service/federation/service"
)

const srvName = "federation"

var log = log15.New("cmd", srvName)
var isShowVersion = flag.Bool("v", false, "show version")

func main() {
	config.RegisterFlags()
	flag.Parse()
	if *isShowVersion {
		fmt.Printf("Project: %s\n", srvName)
		os.Exit(0)
	}
	if err := config.Init(); err != nil {
		panic(err)
	}
	log.Info("config", "clusterId", config.Conf.ClusterID, "grpc", config.Conf.GRPCServer.Addr, "enabled", config.Conf.Enabled)

	svc := service.New(config.Conf)
	rpc := xgrpc.New(config.Conf.GRPCServer.Addr, svc)
	_ = xhttp.Init(svc, config.Conf.AdminHTTPAddr)

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT)
	for {
		s := <-c
		log.Info("signal", "s", s.String())
		switch s {
		case syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT:
			rpc.Shutdown()
			svc.Close()
			time.Sleep(time.Second)
			log.Info(srvName + " exit")
			return
		default:
			return
		}
	}
}
