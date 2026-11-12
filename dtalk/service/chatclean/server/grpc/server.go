package grpc

import (
	"context"

	xgrpc "github.com/web3chat/txchat/dtalk/pkg/net/grpc"
	pb "github.com/web3chat/txchat/dtalk/service/chatclean/api"
	"github.com/web3chat/txchat/dtalk/service/chatclean/service"
)

// New 启动 chatclean gRPC 服务
func New(c *xgrpc.ServerConfig, svc *service.Service) *xgrpc.Server {
	ws := xgrpc.NewServer(c)
	pb.RegisterChatCleanServer(ws.Server(), &server{svc: svc})
	ws, err := ws.Start()
	if err != nil {
		panic(err)
	}
	return ws
}

type server struct {
	pb.UnimplementedChatCleanServer
	svc *service.Service
}

func (s *server) CleanPrivateChat(ctx context.Context, req *pb.CleanPrivateChatReq) (*pb.CleanPrivateChatReply, error) {
	deleted, err := s.svc.CleanPrivateChat(ctx,
		req.OperatorId, req.TargetId,
		service.DateOption(req.DateOpt),
		req.StartTime, req.EndTime,
	)
	if err != nil {
		return nil, err
	}
	return &pb.CleanPrivateChatReply{DeletedCount: deleted}, nil
}

func (s *server) SetAutoDeleteConfig(ctx context.Context, req *pb.SetAutoDeleteConfigReq) (*pb.SetAutoDeleteConfigReply, error) {
	cfg, err := s.svc.SetAutoDeleteConfig(ctx,
		req.OperatorId, req.TargetId,
		service.AutoDeleteOption(req.Option),
	)
	if err != nil {
		return nil, err
	}
	return &pb.SetAutoDeleteConfigReply{
		Option:    pb.AutoDeleteOption(cfg.Option),
		UpdatedBy: cfg.UpdatedBy,
		UpdatedAt: cfg.UpdatedAt,
	}, nil
}

func (s *server) GetAutoDeleteConfig(ctx context.Context, req *pb.GetAutoDeleteConfigReq) (*pb.GetAutoDeleteConfigReply, error) {
	cfg, err := s.svc.GetAutoDeleteConfig(ctx, req.OperatorId, req.TargetId)
	if err != nil {
		return nil, err
	}
	return &pb.GetAutoDeleteConfigReply{
		Option:    pb.AutoDeleteOption(cfg.Option),
		UpdatedBy: cfg.UpdatedBy,
		UpdatedAt: cfg.UpdatedAt,
	}, nil
}
