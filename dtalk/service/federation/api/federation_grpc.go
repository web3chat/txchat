package federation

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	Federation_DeliverPrivate_FullMethodName = "/federation.Federation/DeliverPrivate"
	Federation_MirrorPrivate_FullMethodName  = "/federation.Federation/MirrorPrivate"
	Federation_SyncState_FullMethodName      = "/federation.Federation/SyncState"
	Federation_DeliverGroup_FullMethodName   = "/federation.Federation/DeliverGroup"
	Federation_Ping_FullMethodName           = "/federation.Federation/Ping"
)

type FederationClient interface {
	DeliverPrivate(ctx context.Context, in *DeliverPrivateReq, opts ...grpc.CallOption) (*DeliverPrivateResp, error)
	MirrorPrivate(ctx context.Context, in *MirrorPrivateReq, opts ...grpc.CallOption) (*MirrorPrivateResp, error)
	SyncState(ctx context.Context, in *SyncStateReq, opts ...grpc.CallOption) (*SyncStateResp, error)
	DeliverGroup(ctx context.Context, in *DeliverGroupReq, opts ...grpc.CallOption) (*DeliverGroupResp, error)
	Ping(ctx context.Context, in *PingReq, opts ...grpc.CallOption) (*PingResp, error)
}

type federationClient struct {
	cc grpc.ClientConnInterface
}

func NewFederationClient(cc grpc.ClientConnInterface) FederationClient {
	return &federationClient{cc}
}

func (c *federationClient) DeliverPrivate(ctx context.Context, in *DeliverPrivateReq, opts ...grpc.CallOption) (*DeliverPrivateResp, error) {
	out := new(DeliverPrivateResp)
	err := c.cc.Invoke(ctx, Federation_DeliverPrivate_FullMethodName, in, out, opts...)
	return out, err
}

func (c *federationClient) MirrorPrivate(ctx context.Context, in *MirrorPrivateReq, opts ...grpc.CallOption) (*MirrorPrivateResp, error) {
	out := new(MirrorPrivateResp)
	err := c.cc.Invoke(ctx, Federation_MirrorPrivate_FullMethodName, in, out, opts...)
	return out, err
}

func (c *federationClient) SyncState(ctx context.Context, in *SyncStateReq, opts ...grpc.CallOption) (*SyncStateResp, error) {
	out := new(SyncStateResp)
	err := c.cc.Invoke(ctx, Federation_SyncState_FullMethodName, in, out, opts...)
	return out, err
}

func (c *federationClient) DeliverGroup(ctx context.Context, in *DeliverGroupReq, opts ...grpc.CallOption) (*DeliverGroupResp, error) {
	out := new(DeliverGroupResp)
	err := c.cc.Invoke(ctx, Federation_DeliverGroup_FullMethodName, in, out, opts...)
	return out, err
}

func (c *federationClient) Ping(ctx context.Context, in *PingReq, opts ...grpc.CallOption) (*PingResp, error) {
	out := new(PingResp)
	err := c.cc.Invoke(ctx, Federation_Ping_FullMethodName, in, out, opts...)
	return out, err
}

type FederationServer interface {
	DeliverPrivate(context.Context, *DeliverPrivateReq) (*DeliverPrivateResp, error)
	MirrorPrivate(context.Context, *MirrorPrivateReq) (*MirrorPrivateResp, error)
	SyncState(context.Context, *SyncStateReq) (*SyncStateResp, error)
	DeliverGroup(context.Context, *DeliverGroupReq) (*DeliverGroupResp, error)
	Ping(context.Context, *PingReq) (*PingResp, error)
	mustEmbedUnimplementedFederationServer()
}

type UnimplementedFederationServer struct{}

func (UnimplementedFederationServer) DeliverPrivate(context.Context, *DeliverPrivateReq) (*DeliverPrivateResp, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeliverPrivate not implemented")
}
func (UnimplementedFederationServer) MirrorPrivate(context.Context, *MirrorPrivateReq) (*MirrorPrivateResp, error) {
	return nil, status.Errorf(codes.Unimplemented, "method MirrorPrivate not implemented")
}
func (UnimplementedFederationServer) SyncState(context.Context, *SyncStateReq) (*SyncStateResp, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SyncState not implemented")
}
func (UnimplementedFederationServer) DeliverGroup(context.Context, *DeliverGroupReq) (*DeliverGroupResp, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeliverGroup not implemented")
}
func (UnimplementedFederationServer) Ping(context.Context, *PingReq) (*PingResp, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Ping not implemented")
}
func (UnimplementedFederationServer) mustEmbedUnimplementedFederationServer() {}

type UnsafeFederationServer interface {
	mustEmbedUnimplementedFederationServer()
}

func RegisterFederationServer(s grpc.ServiceRegistrar, srv FederationServer) {
	s.RegisterService(&Federation_ServiceDesc, srv)
}

func _Federation_DeliverPrivate_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeliverPrivateReq)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(FederationServer).DeliverPrivate(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: Federation_DeliverPrivate_FullMethodName}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(FederationServer).DeliverPrivate(ctx, req.(*DeliverPrivateReq))
	}
	return interceptor(ctx, in, info, handler)
}

func _Federation_MirrorPrivate_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(MirrorPrivateReq)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(FederationServer).MirrorPrivate(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: Federation_MirrorPrivate_FullMethodName}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(FederationServer).MirrorPrivate(ctx, req.(*MirrorPrivateReq))
	}
	return interceptor(ctx, in, info, handler)
}

func _Federation_SyncState_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SyncStateReq)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(FederationServer).SyncState(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: Federation_SyncState_FullMethodName}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(FederationServer).SyncState(ctx, req.(*SyncStateReq))
	}
	return interceptor(ctx, in, info, handler)
}

func _Federation_DeliverGroup_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeliverGroupReq)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(FederationServer).DeliverGroup(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: Federation_DeliverGroup_FullMethodName}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(FederationServer).DeliverGroup(ctx, req.(*DeliverGroupReq))
	}
	return interceptor(ctx, in, info, handler)
}

func _Federation_Ping_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PingReq)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(FederationServer).Ping(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: Federation_Ping_FullMethodName}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(FederationServer).Ping(ctx, req.(*PingReq))
	}
	return interceptor(ctx, in, info, handler)
}

var Federation_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "federation.Federation",
	HandlerType: (*FederationServer)(nil),
	Methods: []grpc.MethodDesc{
		{MethodName: "DeliverPrivate", Handler: _Federation_DeliverPrivate_Handler},
		{MethodName: "MirrorPrivate", Handler: _Federation_MirrorPrivate_Handler},
		{MethodName: "SyncState", Handler: _Federation_SyncState_Handler},
		{MethodName: "DeliverGroup", Handler: _Federation_DeliverGroup_Handler},
		{MethodName: "Ping", Handler: _Federation_Ping_Handler},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "federation.proto",
}
