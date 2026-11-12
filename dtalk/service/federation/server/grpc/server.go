package grpc

import (
	"net"
	"time"

	"github.com/inconshreveable/log15"
	pb "github.com/web3chat/txchat/dtalk/service/federation/api"
	"github.com/web3chat/txchat/dtalk/service/federation/service"
	"google.golang.org/grpc"
)

var log = log15.New("module", "federation/grpc")

type Server struct {
	*grpc.Server
	lis net.Listener
}

func New(addr string, svc *service.Service) *Server {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		panic(err)
	}
	s := grpc.NewServer(grpc.ConnectionTimeout(time.Duration(svc.Config().GRPCServer.Timeout)))
	pb.RegisterFederationServer(s, svc)
	go func() {
		log.Info("federation grpc listen", "addr", addr)
		if err := s.Serve(lis); err != nil {
			log.Error("Serve", "err", err)
		}
	}()
	return &Server{Server: s, lis: lis}
}

func (s *Server) Shutdown() {
	s.GracefulStop()
}
