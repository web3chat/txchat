package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Terry-Mao/goim/pkg/ip"
	"github.com/rs/zerolog/log"
	"github.com/web3chat/txchat/dtalk/pkg/naming"
	"github.com/web3chat/txchat/dtalk/service/chatclean/config"
	"github.com/web3chat/txchat/dtalk/service/chatclean/scheduler"
	grpcServer "github.com/web3chat/txchat/dtalk/service/chatclean/server/grpc"
	"github.com/web3chat/txchat/dtalk/service/chatclean/service"
	xlog "github.com/txchat/im-pkg/log"
)

const srvName = "chatclean"

var (
	projectVersion = "0.1.0"
	isShowVersion  = flag.Bool("v", false, "show project version")
)

func main() {
	flag.Parse()
	if *isShowVersion {
		fmt.Printf("Project: %s\nVersion: %s\n", srvName, projectVersion)
		os.Exit(0)
	}

	if err := config.Init(); err != nil {
		panic(err)
	}

	logger, err := xlog.Init(config.Conf.Log)
	if err != nil {
		panic(err)
	}
	log.Logger = logger.With().Str("service", srvName).Logger()

	svc := service.New(config.Conf)
	sch := scheduler.New(config.Conf, svc)
	sch.Start()

	srv := grpcServer.New(config.Conf.GRPCServer, svc)

	if config.Conf.Reg == nil {
		log.Fatal().Msg("config Reg is nil, cannot register to etcd")
	}
	var registerAddr string
	if config.Conf.Reg.RegisterAddr != "" {
		registerAddr = config.Conf.Reg.RegisterAddr
	} else {
		registerAddr, err = registerListenAddr(config.Conf.GRPCServer.Addr)
		if err != nil {
			log.Fatal().Err(err).Str("grpcAddr", config.Conf.GRPCServer.Addr).Msg("parse gRPC listen addr for etcd registration")
		}
	}
	const leaseTTL = int64(15)
	if err := naming.Register(config.Conf.Reg.RegAddrs, config.Conf.Reg.SrvName, registerAddr, config.Conf.Reg.Schema, leaseTTL); err != nil {
		log.Fatal().Err(err).
			Str("regAddrs", config.Conf.Reg.RegAddrs).
			Str("srvName", config.Conf.Reg.SrvName).
			Str("schema", config.Conf.Reg.Schema).
			Str("registerAddr", registerAddr).
			Int64("leaseTTL", leaseTTL).
			Msg("naming.Register failed")
	}
	log.Info().
		Str("name", srvName).
		Str("version", projectVersion).
		Str("registerAddr", registerAddr).
		Str("etcd", config.Conf.Reg.RegAddrs).
		Str("srvName", config.Conf.Reg.SrvName).
		Str("schema", config.Conf.Reg.Schema).
		Msg("service started and registered to etcd")

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT)
	for {
		s := <-c
		log.Info().Str("signal", s.String()).Msg("service got signal")
		switch s {
		case syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT:
			sch.Stop()
			if err := naming.UnRegister(config.Conf.Reg.SrvName, registerAddr, config.Conf.Reg.Schema); err != nil {
				log.Error().Err(err).
					Str("registerAddr", registerAddr).
					Str("srvName", config.Conf.Reg.SrvName).
					Str("schema", config.Conf.Reg.Schema).
					Msg("naming.UnRegister")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := srv.Shutdown(ctx); err != nil {
				log.Error().Err(err).Msg("server shutdown error")
			}
			svc.Shutdown(ctx)
			log.Info().Str("name", srvName).Msg("service exited")
			return
		case syscall.SIGHUP:
			// TODO: hot reload config
		default:
			return
		}
	}
}

// registerListenAddr 将 gRPC 监听地址转换为写入 etcd 的「对客户端可达」地址（本机 IP + 端口），与其它 dtalk 服务一致。
func registerListenAddr(listenAddr string) (string, error) {
	_, port, err := net.SplitHostPort(listenAddr)
	if err != nil {
		if strings.HasPrefix(listenAddr, ":") && len(listenAddr) > 1 {
			port = strings.TrimPrefix(listenAddr, ":")
		} else {
			return "", fmt.Errorf("split host port: %w", err)
		}
	}
	if port == "" {
		return "", fmt.Errorf("empty port from listen addr %q", listenAddr)
	}
	return fmt.Sprintf("%s:%s", ip.InternalIP(), port), nil
}
