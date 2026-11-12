package grpc

import (
	"context"
	"net"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/txchat/im-pkg/trace"

	pb "github.com/web3chat/txchat/im/api/comet/grpc"
	"github.com/web3chat/txchat/im/comet"
	"github.com/web3chat/txchat/im/comet/conf"
	"github.com/web3chat/txchat/im/comet/errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

// New comet grpc server.
func New(c *conf.RPCServer, s *comet.Comet) *grpc.Server {
	keepParams := grpc.KeepaliveParams(keepalive.ServerParameters{
		MaxConnectionIdle:     time.Duration(c.IdleTimeout),
		MaxConnectionAgeGrace: time.Duration(c.ForceCloseWait),
		Time:                  time.Duration(c.KeepAliveInterval),
		Timeout:               time.Duration(c.KeepAliveTimeout),
		MaxConnectionAge:      time.Duration(c.MaxLifeTime),
	})
	connectionTimeout := grpc.ConnectionTimeout(time.Duration(c.Timeout))
	srv := grpc.NewServer(keepParams, connectionTimeout,
		grpc.ChainUnaryInterceptor(
			trace.OpentracingServerInterceptor,
		))
	pb.RegisterCometServer(srv, &server{srv: s})
	lis, err := net.Listen(c.Network, c.Addr)
	if err != nil {
		panic(err)
	}
	go func() {
		if err := srv.Serve(lis); err != nil {
			panic(err)
		}
	}()
	return srv
}

type server struct {
	pb.UnimplementedCometServer
	srv *comet.Comet
}

// PushMsg push a message to specified sub keys.
func (s *server) PushMsg(ctx context.Context, req *pb.PushMsgReq) (reply *pb.PushMsgReply, err error) {
	if len(req.Keys) == 0 || req.Proto == nil {
		return nil, errors.ErrPushMsgArg
	}
	index := make(map[string]int32)
	var seq int32
	log.Info().Strs("keys", req.Keys).Int32("op", req.Proto.Op).Int32("seq", req.Proto.Seq).Msg("PushMsg start")
	for _, key := range req.Keys {
		bucket := s.srv.Bucket(key)
		channel := bucket.Channel(key)
		if channel != nil {
			if seq, err = channel.Push(req.Proto); err != nil {
				log.Error().Err(err).Str("key", key).Int32("op", req.Proto.Op).Int32("seq", req.Proto.Seq).Msg("PushMsg channel.Push failed")
				return
			}
			index[key] = seq
			log.Info().Str("PushMsg key", key).Int32("seq", seq).Int32("op", req.Proto.Op).Int32("protoSeq", req.Proto.Seq).Msg("push msg success")
		} else {
			log.Warn().Str("key", key).Int32("op", req.Proto.Op).Int32("seq", req.Proto.Seq).Msg("PushMsg channel is nil, key not found in bucket")
		}
	}
	log.Info().Strs("keys", req.Keys).Interface("index", index).Int("indexLen", len(index)).Msg("PushMsg completed")
	if len(index) == 0 {
		log.Error().Msg("PushMsg  index is empty")
		return nil, errors.ErrPushMsgArg
	}
	return &pb.PushMsgReply{Index: index}, nil
}

// Broadcast broadcast msg to all user.
func (s *server) Broadcast(ctx context.Context, req *pb.BroadcastReq) (*pb.BroadcastReply, error) {
	if req.Proto == nil {
		return nil, errors.ErrBroadCastArg
	}
	// TODO use broadcast queue
	go func() {
		for _, bucket := range s.srv.Buckets() {
			bucket.Broadcast(req.GetProto(), req.ProtoOp)
		}
	}()
	return &pb.BroadcastReply{}, nil
}

func (s *server) BroadcastGroup(ctx context.Context, req *pb.BroadcastGroupReq) (*pb.BroadcastGroupReply, error) {
	if req.Proto == nil || req.GroupID == "" {
		return nil, errors.ErrBroadCastArg
	}
	for _, bucket := range s.srv.Buckets() {
		bucket.BroadcastGroup(req)
	}
	return &pb.BroadcastGroupReply{}, nil
}

func (s *server) JoinGroups(ctx context.Context, req *pb.JoinGroupsReq) (*pb.JoinGroupsReply, error) {
	if len(req.Keys) == 0 || len(req.Gid) == 0 {
		return nil, errors.ErrJoinGroupArg
	}
	for _, key := range req.Keys {
		var channel *comet.Channel
		bucket := s.srv.Bucket(key)
		if channel = bucket.Channel(key); channel == nil {
			log.Error().Str("key", key).Msg("JoinGroups get channel err")
			continue
			//return &pb.JoinGroupsReply{}, errors.ErrUnconnected todo 2021_12_08_14_36:
		}
		for _, gid := range req.Gid {
			var group *comet.Group
			if group = bucket.Group(gid); group == nil {
				group, _ = bucket.PutGroup(gid)
			}
			err := group.Put(channel)
			if err != nil {
				log.Error().Err(err).Str("key", key).Str("gid", gid).
					Int32("channel.Seq", channel.Seq).Str("channel.Key", channel.Key).Str("channel.Ip", channel.IP).Str("channel.Port", channel.Port).
					Msg("JoinGroups get channel err")
				continue
				//return &pb.JoinGroupsReply{}, err todo 2021_12_08_14_36:
			}
		}
	}
	return &pb.JoinGroupsReply{}, nil
}

func (s *server) LeaveGroups(ctx context.Context, req *pb.LeaveGroupsReq) (*pb.LeaveGroupsReply, error) {
	if len(req.Keys) == 0 || len(req.Gid) == 0 {
		return nil, errors.ErrPushMsgArg
	}
	for _, key := range req.Keys {
		var channel *comet.Channel
		bucket := s.srv.Bucket(key)
		if channel = bucket.Channel(key); channel == nil {
			continue
			//return &pb.LeaveGroupsReply{}, errors.ErrUnconnected todo 2021_12_08_14_36:
		}
		for _, gid := range req.Gid {
			if group := bucket.Group(gid); group != nil {
				group.Del(channel)
			}
		}
	}
	return &pb.LeaveGroupsReply{}, nil
}

func (s *server) DelGroups(ctx context.Context, req *pb.DelGroupsReq) (*pb.DelGroupsReply, error) {
	for _, gid := range req.Gid {
		for _, bucket := range s.srv.Buckets() {
			if g := bucket.Group(gid); g != nil {
				bucket.DelGroup(g)
			}
		}
	}
	return &pb.DelGroupsReply{}, nil
}
