package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/inconshreveable/log15"
	"github.com/web3chat/txchat/dtalk/pkg/naming"
	"github.com/web3chat/txchat/dtalk/pkg/presenceclient"
	pb "github.com/web3chat/txchat/dtalk/service/federation/api"
	"github.com/web3chat/txchat/dtalk/service/federation/config"
	"github.com/web3chat/txchat/dtalk/service/federation/dao"
	"github.com/web3chat/txchat/dtalk/service/record/store/model"
	comet "github.com/web3chat/txchat/im/api/comet/grpc"
	logic "github.com/web3chat/txchat/im/api/logic/grpc"
	"github.com/web3chat/txchat/imparse/chat"
	xproto "github.com/web3chat/txchat/imparse/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/status"
)

type Service struct {
	pb.UnimplementedFederationServer
	cfg          *config.Config
	dao          *dao.Dao
	log          log15.Logger
	presence     *presenceclient.Client
	logicClient  logic.LogicClient
	clusterCache map[string]string // clusterId -> bridge addr
	cacheMu      sync.RWMutex
	stopCh       chan struct{}
	enabled      bool
	leaseOwner   string
}

func New(cfg *config.Config) *Service {
	host, _ := os.Hostname()
	if host == "" {
		host = "federation"
	}
	s := &Service{
		cfg:          cfg,
		dao:          dao.New(cfg),
		log:          log15.New("module", "federation/service"),
		presence:     presenceclient.New(cfg.Presence.URL, time.Duration(cfg.Presence.Timeout)),
		clusterCache: make(map[string]string),
		stopCh:       make(chan struct{}),
		enabled:      cfg.Enabled,
		leaseOwner:   fmt.Sprintf("%s-%d", host, os.Getpid()),
	}
	s.initLogicClient()
	go s.refreshClustersLoop()
	go s.outboxLoop()
	return s
}

func (s *Service) Config() *config.Config { return s.cfg }

func (s *Service) Enabled() bool { return s.enabled }

func (s *Service) SetEnabled(v bool) { s.enabled = v }

func (s *Service) Close() {
	close(s.stopCh)
	if s.dao != nil {
		s.dao.Close()
	}
}

func (s *Service) initLogicClient() {
	if s.cfg.LogicRPC == nil || s.cfg.LogicRPC.RegAddrs == "" {
		s.log.Error("logic RPC not configured")
		return
	}
	rb := naming.NewResolver(s.cfg.LogicRPC.RegAddrs, s.cfg.LogicRPC.Schema)
	resolver.Register(rb)
	addr := fmt.Sprintf("%s:///%s", s.cfg.LogicRPC.Schema, s.cfg.LogicRPC.SrvName)
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		s.log.Error("dial logic", "addr", addr, "err", err)
		return
	}
	s.logicClient = logic.NewLogicClient(conn)
}

func (s *Service) refreshClustersLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	s.refreshClusters()
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.refreshClusters()
		}
	}
}

func (s *Service) refreshClusters() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.cfg.Presence.Timeout))
	defer cancel()
	list, err := s.presence.ListClusters(ctx)
	if err != nil {
		s.log.Error("refreshClusters", "err", err)
		return
	}
	m := make(map[string]string, len(list))
	for _, c := range list {
		if c.Status == 1 && c.BridgeGRPCAddr != "" {
			m[c.ClusterID] = c.BridgeGRPCAddr
		}
	}
	s.cacheMu.Lock()
	s.clusterCache = m
	s.cacheMu.Unlock()
}

func (s *Service) bridgeAddr(clusterID string) (string, bool) {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()
	addr, ok := s.clusterCache[clusterID]
	return addr, ok
}

func (s *Service) IsWhitelisted(clusterID string) bool {
	_, ok := s.bridgeAddr(clusterID)
	return ok
}

func (s *Service) Ping(ctx context.Context, req *pb.PingReq) (*pb.PingResp, error) {
	return &pb.PingResp{ClusterId: s.cfg.EffectiveClusterID(), Ok: true}, nil
}

func (s *Service) DeliverPrivate(ctx context.Context, req *pb.DeliverPrivateReq) (*pb.DeliverPrivateResp, error) {
	if !s.enabled {
		return &pb.DeliverPrivateResp{Delivered: false, Message: "federation disabled"}, nil
	}
	if req.GetHop() > 1 {
		s.log.Error("DeliverPrivate hop exceeded", "hop", req.GetHop(), "mid", req.GetMid(), "source", req.GetSourceCluster())
		return &pb.DeliverPrivateResp{Delivered: false, Message: "hop exceeded"}, nil
	}
	if !s.IsWhitelisted(req.GetSourceCluster()) {
		s.log.Error("DeliverPrivate source not whitelisted", "source", req.GetSourceCluster(), "mid", req.GetMid())
		return &pb.DeliverPrivateResp{Delivered: false, Message: "source not whitelisted"}, nil
	}
	key := req.GetIdempotencyKey()
	if key == "" {
		key = fmt.Sprintf("%s:%d", req.GetSourceCluster(), req.GetMid())
	}
	exists, err := s.dao.InboxExists(key)
	if err != nil {
		return nil, err
	}
	stored := false
	if !exists {
		if req.GetNeedStore() {
			if err := s.storeFromMsg(req); err != nil {
				s.log.Error("DeliverPrivate store", "mid", req.GetMid(), "from", req.GetFrom(), "target", req.GetTarget(), "err", err)
				return nil, err
			}
			stored = true
		}
		if _, err := s.dao.InsertInbox(key, req.GetSourceCluster(), req.GetMid(), req.GetAppId(), req.GetFrom(), req.GetTarget(), dao.OpDeliverPrivate, "", stored); err != nil {
			return nil, err
		}
	} else {
		stored = true
	}

	online := false
	if s.logicClient != nil && len(req.GetMsg()) > 0 {
		reply, perr := s.logicClient.PushByMids(ctx, &logic.MidsMsg{
			AppId: req.GetAppId(),
			ToIds: []string{req.GetTarget()},
			Msg:   req.GetMsg(),
		})
		if perr != nil {
			s.log.Error("DeliverPrivate PushByMids", "mid", req.GetMid(), "target", req.GetTarget(), "err", perr)
		} else if reply != nil && reply.GetIsOk() && len(reply.GetMsg()) > 0 {
			var index comet.PushMsgReply
			if uerr := proto.Unmarshal(reply.GetMsg(), &index); uerr == nil && len(index.Index) > 0 {
				online = true
			}
		}
	}
	return &pb.DeliverPrivateResp{Delivered: true, Online: online, Stored: stored}, nil
}

// MirrorPrivate 将接收方已落库消息镜像到发送方归属集群，并对 from 多端推送。
func (s *Service) MirrorPrivate(ctx context.Context, req *pb.MirrorPrivateReq) (*pb.MirrorPrivateResp, error) {
	if !s.enabled {
		return nil, status.Error(codes.FailedPrecondition, "federation disabled")
	}
	if req.GetHop() != 1 {
		s.log.Error("MirrorPrivate hop invalid", "hop", req.GetHop(), "mid", req.GetMid(), "source", req.GetSourceCluster())
		return nil, status.Error(codes.FailedPrecondition, "HOP_EXCEEDED")
	}
	if !s.IsWhitelisted(req.GetSourceCluster()) {
		s.log.Error("MirrorPrivate source not allowed", "source", req.GetSourceCluster(), "mid", req.GetMid())
		return nil, status.Error(codes.Unauthenticated, "SOURCE_NOT_ALLOWED")
	}
	if req.GetFrameType() != "" && req.GetFrameType() != string(chat.PrivateFrameType) {
		return nil, status.Error(codes.InvalidArgument, "INVALID_PAYLOAD: frame_type")
	}
	if req.GetAppId() == "" || req.GetFrom() == "" || req.GetTarget() == "" || req.GetMid() == 0 || len(req.GetMsg()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "INVALID_PAYLOAD")
	}
	key := req.GetIdempotencyKey()
	if key == "" {
		key = fmt.Sprintf("%s:mirror:%d", req.GetSourceCluster(), req.GetMid())
	}

	parsed, err := parseCommonFromPushMsg(req.GetMsg())
	if err != nil {
		s.log.Error("MirrorPrivate parse msg", "mid", req.GetMid(), "source", req.GetSourceCluster(), "err", err)
		return nil, status.Error(codes.InvalidArgument, "INVALID_PAYLOAD: msg")
	}
	if parsed.mid != 0 && parsed.mid != req.GetMid() {
		return nil, status.Error(codes.InvalidArgument, "INVALID_PAYLOAD: mid mismatch")
	}
	if parsed.from != "" && parsed.from != req.GetFrom() {
		return nil, status.Error(codes.InvalidArgument, "INVALID_PAYLOAD: from mismatch")
	}
	if parsed.target != "" && parsed.target != req.GetTarget() {
		return nil, status.Error(codes.InvalidArgument, "INVALID_PAYLOAD: target mismatch")
	}
	seq := req.GetSeq()
	if seq == "" {
		seq = parsed.seq
	}
	createTime := parsed.createTime
	if createTime == 0 {
		createTime = uint64(time.Now().UnixMilli())
	}
	contentHash := req.GetContentHash()
	if contentHash == "" {
		contentHash = dao.ContentFingerprint(req.GetAppId(), req.GetMid(), seq, req.GetFrom(), req.GetTarget(),
			parsed.msgType, createTime, parsed.content, parsed.source, parsed.reference)
	}

	duplicated, err := s.dao.StoreMirroredPrivate(dao.MirrorPrivateInput{
		IdempotencyKey: key,
		SourceCluster:  req.GetSourceCluster(),
		AppID:          req.GetAppId(),
		From:           req.GetFrom(),
		Target:         req.GetTarget(),
		Mid:            req.GetMid(),
		Seq:            seq,
		MsgType:        parsed.msgType,
		Content:        parsed.content,
		Source:         parsed.source,
		Reference:      parsed.reference,
		CreateTime:     createTime,
		ContentHash:    contentHash,
	})
	if err != nil {
		if errors.Is(err, dao.ErrIdempotencyConflict) {
			return nil, status.Error(codes.AlreadyExists, "IDEMPOTENCY_CONFLICT")
		}
		if errors.Is(err, dao.ErrMidCollision) {
			return nil, status.Error(codes.Aborted, "MID_COLLISION")
		}
		s.log.Error("MirrorPrivate store", "mid", req.GetMid(), "from", req.GetFrom(), "target", req.GetTarget(),
			"source", req.GetSourceCluster(), "key", key, "err", err)
		return nil, status.Error(codes.Internal, "STORE_FAILED")
	}
	if duplicated {
		s.log.Info("MirrorPrivate duplicated", "mid", req.GetMid(), "key", key, "source", req.GetSourceCluster())
		return &pb.MirrorPrivateResp{
			Delivered: true, Stored: false, Duplicated: true, NotifyQueued: false, Message: "already processed",
		}, nil
	}

	notifyQueued := false
	if s.logicClient != nil {
		_, perr := s.logicClient.PushByMids(ctx, &logic.MidsMsg{
			AppId: req.GetAppId(),
			ToIds: []string{req.GetFrom()},
			Msg:   req.GetMsg(),
		})
		if perr != nil {
			s.log.Error("MirrorPrivate PushByMids from", "mid", req.GetMid(), "from", req.GetFrom(),
				"appId", req.GetAppId(), "key", key, "err", perr)
		} else {
			notifyQueued = true
		}
	}
	s.log.Info("MirrorPrivate ok", "mid", req.GetMid(), "from", req.GetFrom(), "target", req.GetTarget(),
		"source", req.GetSourceCluster(), "key", key, "notify", notifyQueued)
	return &pb.MirrorPrivateResp{
		Delivered: true, Stored: true, Duplicated: false, NotifyQueued: notifyQueued, Message: "ok",
	}, nil
}

type parsedCommon struct {
	mid        int64
	seq        string
	from       string
	target     string
	msgType    uint32
	content    string
	source     string
	reference  string
	createTime uint64
}

func parseCommonFromPushMsg(msg []byte) (*parsedCommon, error) {
	out := &parsedCommon{}
	var protoMsg comet.Proto
	if err := proto.Unmarshal(msg, &protoMsg); err != nil {
		return nil, err
	}
	var biz xproto.Proto
	if err := proto.Unmarshal(protoMsg.Body, &biz); err != nil {
		return nil, err
	}
	if biz.GetEventType() != xproto.Proto_common {
		return out, nil
	}
	var common xproto.Common
	if err := proto.Unmarshal(biz.GetBody(), &common); err != nil {
		return nil, err
	}
	out.mid = common.GetMid()
	out.seq = common.GetSeq()
	out.from = common.GetFrom()
	out.target = common.GetTarget()
	out.msgType = uint32(common.GetMsgType())
	out.createTime = common.GetDatetime()
	out.content = string(model.ParseCommonMsg(&common))
	out.source = string(model.ParseSource(&common))
	out.reference = string(model.ParseReference(&common))
	return out, nil
}

func (s *Service) storeFromMsg(req *pb.DeliverPrivateReq) error {
	msgType := uint32(0)
	content := ""
	source := ""
	reference := ""
	createTime := uint64(time.Now().UnixMilli())
	seq := req.GetSeq()

	var protoMsg comet.Proto
	if err := proto.Unmarshal(req.GetMsg(), &protoMsg); err == nil {
		var biz xproto.Proto
		if err := proto.Unmarshal(protoMsg.Body, &biz); err == nil && biz.GetEventType() == xproto.Proto_common {
			var common xproto.Common
			if err := proto.Unmarshal(biz.GetBody(), &common); err == nil {
				msgType = uint32(common.GetMsgType())
				createTime = common.GetDatetime()
				if seq == "" {
					seq = common.GetSeq()
				}
				content = string(model.ParseCommonMsg(&common))
				source = string(model.ParseSource(&common))
				reference = string(model.ParseReference(&common))
			}
		}
	}
	return s.dao.AppendFederatedMsg(req.GetMid(), seq, req.GetFrom(), req.GetTarget(), msgType, content, source, reference, req.GetSourceCluster(), createTime)
}

func (s *Service) SyncState(ctx context.Context, req *pb.SyncStateReq) (*pb.SyncStateResp, error) {
	if !s.enabled {
		return &pb.SyncStateResp{Ok: false, Message: "federation disabled"}, nil
	}
	if req.GetHop() > 1 {
		return &pb.SyncStateResp{Ok: false, Message: "hop exceeded"}, nil
	}
	if !s.IsWhitelisted(req.GetSourceCluster()) {
		return &pb.SyncStateResp{Ok: false, Message: "source not whitelisted"}, nil
	}
	key := req.GetIdempotencyKey()
	if key == "" {
		key = fmt.Sprintf("%s:%s:%d", req.GetSourceCluster(), req.GetOp(), req.GetMid())
	}
	exists, err := s.dao.InboxExists(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		switch req.GetOp() {
		case "revoke":
			if err := s.dao.MarkMsgRevoked(req.GetMid()); err != nil {
				return nil, err
			}
		default:
			s.log.Error("SyncState unknown op", "op", req.GetOp(), "mid", req.GetMid())
		}
		if _, err := s.dao.InsertInbox(key, req.GetSourceCluster(), req.GetMid(), req.GetAppId(), req.GetFrom(), req.GetTarget(), dao.OpSyncState+":"+req.GetOp(), "", true); err != nil {
			return nil, err
		}
	}
	if s.logicClient != nil && len(req.GetPayload()) > 0 {
		_, _ = s.logicClient.PushByMids(ctx, &logic.MidsMsg{
			AppId: req.GetAppId(),
			ToIds: []string{req.GetTarget(), req.GetFrom()},
			Msg:   req.GetPayload(),
		})
	}
	return &pb.SyncStateResp{Ok: true}, nil
}

func (s *Service) DeliverGroup(ctx context.Context, req *pb.DeliverGroupReq) (*pb.DeliverGroupResp, error) {
	if !s.enabled {
		return &pb.DeliverGroupResp{Delivered: false, Message: "federation disabled"}, nil
	}
	if req.GetHop() > 1 || !s.IsWhitelisted(req.GetSourceCluster()) {
		return &pb.DeliverGroupResp{Delivered: false, Message: "rejected"}, nil
	}
	key := req.GetIdempotencyKey()
	if key == "" {
		key = fmt.Sprintf("%s:g:%d", req.GetSourceCluster(), req.GetMid())
	}
	exists, err := s.dao.InboxExists(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		_, _ = s.dao.InsertInbox(key, req.GetSourceCluster(), req.GetMid(), req.GetAppId(), req.GetFrom(), req.GetGroupId(), dao.OpDeliverGroup, "", false)
	}
	if s.logicClient != nil && len(req.GetMemberUids()) > 0 && len(req.GetMsg()) > 0 {
		_, err = s.logicClient.PushByMids(ctx, &logic.MidsMsg{
			AppId: req.GetAppId(),
			ToIds: req.GetMemberUids(),
			Msg:   req.GetMsg(),
		})
		if err != nil {
			s.log.Error("DeliverGroup PushByMids", "mid", req.GetMid(), "group", req.GetGroupId(), "err", err)
			return &pb.DeliverGroupResp{Delivered: false, Message: err.Error()}, nil
		}
	}
	return &pb.DeliverGroupResp{Delivered: true}, nil
}

// EnqueueOrDeliver 源集群：先尝试同步投递，失败写入 outbox
func (s *Service) EnqueueOrDeliver(ctx context.Context, destCluster string, req *pb.DeliverPrivateReq) error {
	if !s.enabled {
		return fmt.Errorf("federation disabled")
	}
	if err := s.deliverToCluster(ctx, destCluster, req); err != nil {
		s.log.Error("EnqueueOrDeliver sync failed, write outbox",
			"dest", destCluster, "mid", req.GetMid(), "from", req.GetFrom(), "target", req.GetTarget(),
			"key", req.GetIdempotencyKey(), "err", err)
		return s.dao.InsertOutbox(&dao.OutboxRow{
			IdempotencyKey: req.GetIdempotencyKey(),
			OpType:         dao.OpDeliverPrivate,
			DestClusterID:  destCluster,
			AppID:          req.GetAppId(),
			FromUID:        req.GetFrom(),
			TargetUID:      req.GetTarget(),
			Mid:            req.GetMid(),
			Seq:            req.GetSeq(),
			FrameType:      req.GetFrameType(),
			Payload:        mustMarshalDeliver(req),
			NeedStore:      req.GetNeedStore(),
		})
	}
	return nil
}

func mustMarshalDeliver(req *pb.DeliverPrivateReq) []byte {
	b, err := proto.Marshal(req)
	if err != nil {
		return nil
	}
	return b
}

func (s *Service) deliverToCluster(ctx context.Context, destCluster string, req *pb.DeliverPrivateReq) error {
	addr, ok := s.bridgeAddr(destCluster)
	if !ok {
		s.refreshClusters()
		addr, ok = s.bridgeAddr(destCluster)
		if !ok {
			return fmt.Errorf("unknown dest cluster %s", destCluster)
		}
	}
	dialCtx, cancel := context.WithTimeout(ctx, time.Duration(s.cfg.Outbox.RPCTimeout))
	defer cancel()
	conn, err := grpc.DialContext(dialCtx, addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		return err
	}
	defer conn.Close()
	cli := pb.NewFederationClient(conn)
	resp, err := cli.DeliverPrivate(dialCtx, req)
	if err != nil {
		return err
	}
	if resp == nil || !resp.GetDelivered() {
		return fmt.Errorf("deliver rejected: %s", resp.GetMessage())
	}
	return nil
}

func (s *Service) SyncStateToCluster(ctx context.Context, destCluster string, req *pb.SyncStateReq) error {
	addr, ok := s.bridgeAddr(destCluster)
	if !ok {
		return fmt.Errorf("unknown dest cluster %s", destCluster)
	}
	dialCtx, cancel := context.WithTimeout(ctx, time.Duration(s.cfg.Outbox.RPCTimeout))
	defer cancel()
	conn, err := grpc.DialContext(dialCtx, addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		return err
	}
	defer conn.Close()
	cli := pb.NewFederationClient(conn)
	resp, err := cli.SyncState(dialCtx, req)
	if err != nil {
		return err
	}
	if resp == nil || !resp.GetOk() {
		return fmt.Errorf("sync rejected: %v", resp)
	}
	return nil
}

func (s *Service) LookupClusters(ctx context.Context, appID, uid string) ([]string, error) {
	items, err := s.presence.Get(ctx, appID, uid)
	if err != nil {
		return nil, err
	}
	self := s.cfg.EffectiveClusterID()
	out := make([]string, 0)
	seen := map[string]struct{}{}
	for _, it := range items {
		for _, c := range it.Clusters {
			if c.ClusterID == "" || c.ClusterID == self {
				continue
			}
			if _, ok := seen[c.ClusterID]; ok {
				continue
			}
			seen[c.ClusterID] = struct{}{}
			out = append(out, c.ClusterID)
		}
	}
	return out, nil
}

func (s *Service) outboxLoop() {
	interval := time.Duration(s.cfg.Outbox.PollInterval)
	if interval <= 0 {
		interval = 2 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.processOutbox()
		}
	}
}

func (s *Service) processOutbox() {
	if !s.enabled {
		return
	}
	now := time.Now().UnixMilli()
	leaseMs := int64(time.Duration(s.cfg.Outbox.LeaseDuration) / time.Millisecond)
	if leaseMs <= 0 {
		leaseMs = 30_000
	}
	rows, err := s.dao.ClaimPending(s.cfg.Outbox.BatchSize, now, s.leaseOwner, leaseMs)
	if err != nil || len(rows) == 0 {
		return
	}
	maxRetry := s.cfg.Outbox.MaxRetry
	if maxRetry <= 0 {
		maxRetry = 20
	}
	for _, row := range rows {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.cfg.Outbox.RPCTimeout))
		err := s.dispatchOutbox(ctx, row)
		cancel()
		if err == nil {
			_ = s.dao.MarkOutboxDone(row.ID, s.leaseOwner)
			continue
		}
		retry := row.RetryCount + 1
		dead := retry >= maxRetry
		backoff := time.Duration(retry*retry) * time.Second
		if backoff > 5*time.Minute {
			backoff = 5 * time.Minute
		}
		next := time.Now().Add(backoff).UnixMilli()
		s.log.Error("outbox retry", "id", row.ID, "op", row.OpType, "mid", row.Mid,
			"dest", row.DestClusterID, "from", row.FromUID, "target", row.TargetUID,
			"key", row.IdempotencyKey, "retry", retry, "dead", dead, "err", err)
		_ = s.dao.MarkOutboxRetry(row.ID, retry, next, err.Error(), dead)
	}
}

func (s *Service) dispatchOutbox(ctx context.Context, row *dao.OutboxRow) error {
	op := row.OpType
	if op == "" {
		op = dao.OpDeliverPrivate
	}
	switch op {
	case dao.OpMirrorPrivate:
		return s.processMirrorOutbox(ctx, row)
	default:
		return s.processDeliverOutbox(ctx, row)
	}
}

func (s *Service) processDeliverOutbox(ctx context.Context, row *dao.OutboxRow) error {
	var req pb.DeliverPrivateReq
	if err := proto.Unmarshal(row.Payload, &req); err != nil {
		req = pb.DeliverPrivateReq{
			SourceCluster:  s.cfg.EffectiveClusterID(),
			AppId:          row.AppID,
			From:           row.FromUID,
			Target:         row.TargetUID,
			Mid:            row.Mid,
			Seq:            row.Seq,
			FrameType:      row.FrameType,
			Msg:            row.Payload,
			NeedStore:      row.NeedStore,
			IdempotencyKey: row.IdempotencyKey,
			Hop:            1,
		}
	}
	return s.deliverToCluster(ctx, row.DestClusterID, &req)
}

func (s *Service) processMirrorOutbox(ctx context.Context, row *dao.OutboxRow) error {
	var req pb.MirrorPrivateReq
	if err := proto.Unmarshal(row.Payload, &req); err != nil {
		return fmt.Errorf("unmarshal MirrorPrivateReq: %w", err)
	}
	dest := row.DestClusterID
	if dest == "" {
		clusterID, found, err := s.presence.ResolvePreferredCluster(ctx, row.AppID, row.FromUID)
		if err != nil {
			s.log.Error("ResolvePreferredCluster", "appId", row.AppID, "from", row.FromUID, "mid", row.Mid, "err", err)
			return err
		}
		if !found || clusterID == "" {
			return fmt.Errorf("preferred cluster missing appId=%s from=%s mid=%d", row.AppID, row.FromUID, row.Mid)
		}
		self := s.cfg.EffectiveClusterID()
		if clusterID == self {
			s.log.Info("MirrorPrivate skip self cluster", "cluster", self, "mid", row.Mid, "from", row.FromUID)
			return nil
		}
		if err := s.dao.UpdateOutboxDest(row.ID, clusterID); err != nil {
			return err
		}
		dest = clusterID
		row.DestClusterID = dest
	} else if dest == s.cfg.EffectiveClusterID() {
		s.log.Info("MirrorPrivate skip self cluster", "cluster", dest, "mid", row.Mid, "from", row.FromUID)
		return nil
	}
	return s.mirrorToCluster(ctx, dest, &req)
}

func (s *Service) mirrorToCluster(ctx context.Context, destCluster string, req *pb.MirrorPrivateReq) error {
	addr, ok := s.bridgeAddr(destCluster)
	if !ok {
		s.refreshClusters()
		addr, ok = s.bridgeAddr(destCluster)
		if !ok {
			return fmt.Errorf("unknown dest cluster %s", destCluster)
		}
	}
	dialCtx, cancel := context.WithTimeout(ctx, time.Duration(s.cfg.Outbox.RPCTimeout))
	defer cancel()
	conn, err := grpc.DialContext(dialCtx, addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		return err
	}
	defer conn.Close()
	cli := pb.NewFederationClient(conn)
	resp, err := cli.MirrorPrivate(dialCtx, req)
	if err != nil {
		return err
	}
	if resp == nil || !resp.GetDelivered() {
		return fmt.Errorf("mirror rejected: %s", resp.GetMessage())
	}
	return nil
}
