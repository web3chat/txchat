package outbound

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/inconshreveable/log15"
	"github.com/web3chat/txchat/dtalk/pkg/presenceclient"
	pb "github.com/web3chat/txchat/dtalk/service/federation/api"
	"github.com/web3chat/txchat/dtalk/service/federation/config"
	"github.com/web3chat/txchat/dtalk/service/federation/dao"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client 源集群出站：查 Presence → 远程 Deliver；失败写 outbox
type Client struct {
	log          log15.Logger
	clusterID    string
	enabled      bool
	presence     *presenceclient.Client
	dao          *dao.Dao
	rpcTimeout   time.Duration
	clusterCache map[string]string
	mu           sync.RWMutex
}

func NewFromConfig(cfg *config.Config) *Client {
	if cfg == nil || !cfg.Enabled {
		return &Client{enabled: false, log: log15.New("module", "federation/outbound")}
	}
	c := &Client{
		log:          log15.New("module", "federation/outbound"),
		clusterID:    cfg.EffectiveClusterID(),
		enabled:      true,
		presence:     presenceclient.New(cfg.Presence.URL, time.Duration(cfg.Presence.Timeout)),
		dao:          dao.New(cfg),
		rpcTimeout:   time.Duration(cfg.Outbox.RPCTimeout),
		clusterCache: make(map[string]string),
	}
	if c.rpcTimeout <= 0 {
		c.rpcTimeout = 2 * time.Second
	}
	c.refreshClusters()
	return c
}

func (c *Client) Enabled() bool { return c != nil && c.enabled }

func (c *Client) Close() {
	if c != nil && c.dao != nil {
		c.dao.Close()
	}
}

func (c *Client) refreshClusters() {
	if !c.Enabled() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), c.rpcTimeout)
	defer cancel()
	list, err := c.presence.ListClusters(ctx)
	if err != nil {
		c.log.Error("refreshClusters", "err", err)
		return
	}
	m := make(map[string]string, len(list))
	for _, item := range list {
		if item.Status == 1 && item.BridgeGRPCAddr != "" {
			m[item.ClusterID] = item.BridgeGRPCAddr
		}
	}
	c.mu.Lock()
	c.clusterCache = m
	c.mu.Unlock()
}

func (c *Client) bridgeAddr(clusterID string) (string, bool) {
	c.mu.RLock()
	addr, ok := c.clusterCache[clusterID]
	c.mu.RUnlock()
	return addr, ok
}

// ForwardPrivate 向目标用户所在的异地集群投递；本地 miss 时由 pusher 调用
func (c *Client) ForwardPrivate(ctx context.Context, appID, from, target string, mid int64, seq, frameType string, msg []byte) error {
	if !c.Enabled() {
		return nil
	}
	clusters, err := c.lookupRemoteClusters(ctx, appID, target)
	if err != nil {
		c.log.Error("ForwardPrivate lookup", "appId", appID, "target", target, "mid", mid, "err", err)
		return err
	}
	if len(clusters) == 0 {
		return nil
	}
	var lastErr error
	for _, dest := range clusters {
		key := fmt.Sprintf("%s:%d", c.clusterID, mid)
		req := &pb.DeliverPrivateReq{
			SourceCluster:  c.clusterID,
			AppId:          appID,
			From:           from,
			Target:         target,
			Mid:            mid,
			Seq:            seq,
			FrameType:      frameType,
			Msg:            msg,
			NeedStore:      true,
			IdempotencyKey: key,
			Hop:            1,
		}
		if err := c.deliverOrEnqueue(ctx, dest, req); err != nil {
			c.log.Error("ForwardPrivate deliverOrEnqueue", "dest", dest, "mid", mid, "from", from, "target", target, "key", key, "err", err)
			lastErr = err
		}
	}
	return lastErr
}

// ForwardSyncState 跨集群同步撤回等状态
func (c *Client) ForwardSyncState(ctx context.Context, appID, from, target, op string, mid int64, payload []byte) error {
	if !c.Enabled() {
		return nil
	}
	clusters, err := c.lookupRemoteClusters(ctx, appID, target)
	if err != nil {
		return err
	}
	// 也尝试对方 from 所在集群（多端）
	more, _ := c.lookupRemoteClusters(ctx, appID, from)
	seen := map[string]struct{}{}
	for _, dest := range append(clusters, more...) {
		if _, ok := seen[dest]; ok {
			continue
		}
		seen[dest] = struct{}{}
		req := &pb.SyncStateReq{
			SourceCluster:  c.clusterID,
			AppId:          appID,
			From:           from,
			Target:         target,
			Mid:            mid,
			Op:             op,
			Payload:        payload,
			IdempotencyKey: fmt.Sprintf("%s:%s:%d", c.clusterID, op, mid),
			Hop:            1,
		}
		if err := c.syncState(ctx, dest, req); err != nil {
			c.log.Error("ForwardSyncState", "dest", dest, "op", op, "mid", mid, "err", err)
			last := err
			_ = last
		}
	}
	return nil
}

func (c *Client) lookupRemoteClusters(ctx context.Context, appID, uid string) ([]string, error) {
	items, err := c.presence.Get(ctx, appID, uid)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0)
	seen := map[string]struct{}{}
	preferred := ""
	for _, it := range items {
		if it.PreferredClusterID != "" {
			preferred = it.PreferredClusterID
		}
		for _, cl := range it.Clusters {
			if cl.ClusterID == "" || cl.ClusterID == c.clusterID {
				continue
			}
			if _, ok := seen[cl.ClusterID]; ok {
				continue
			}
			seen[cl.ClusterID] = struct{}{}
			out = append(out, cl.ClusterID)
		}
	}
	// 无在线远程集群时，回退最后登录集群（Presence 粘性记录）
	if len(out) == 0 && preferred != "" && preferred != c.clusterID {
		c.log.Info("lookupRemoteClusters fallback preferredClusterId", "appId", appID, "uid", uid, "preferred", preferred)
		out = append(out, preferred)
	}
	return out, nil
}

func (c *Client) deliverOrEnqueue(ctx context.Context, dest string, req *pb.DeliverPrivateReq) error {
	if err := c.deliver(ctx, dest, req); err != nil {
		payload, _ := proto.Marshal(req)
		return c.dao.InsertOutbox(&dao.OutboxRow{
			IdempotencyKey: req.GetIdempotencyKey(),
			OpType:         dao.OpDeliverPrivate,
			DestClusterID:  dest,
			AppID:          req.GetAppId(),
			FromUID:        req.GetFrom(),
			TargetUID:      req.GetTarget(),
			Mid:            req.GetMid(),
			Seq:            req.GetSeq(),
			FrameType:      req.GetFrameType(),
			Payload:        payload,
			NeedStore:      req.GetNeedStore(),
		})
	}
	return nil
}

// EnqueueMirrorPrivate 仅幂等写入 mirror_private outbox，不查 Presence、不同步拨号。
// dest_cluster_id 允许为空，由 federation worker 解析归属后投递。
func (c *Client) EnqueueMirrorPrivate(ctx context.Context, req *pb.MirrorPrivateReq) error {
	if !c.Enabled() {
		return nil
	}
	if req == nil {
		return fmt.Errorf("nil MirrorPrivateReq")
	}
	key := req.GetIdempotencyKey()
	if key == "" {
		key = fmt.Sprintf("%s:mirror:%d", c.clusterID, req.GetMid())
		req.IdempotencyKey = key
	}
	if req.GetSourceCluster() == "" {
		req.SourceCluster = c.clusterID
	}
	if req.GetHop() == 0 {
		req.Hop = 1
	}
	payload, err := proto.Marshal(req)
	if err != nil {
		c.log.Error("EnqueueMirrorPrivate marshal", "mid", req.GetMid(), "from", req.GetFrom(), "target", req.GetTarget(), "err", err)
		return err
	}
	if err := c.dao.InsertOutbox(&dao.OutboxRow{
		IdempotencyKey: key,
		OpType:         dao.OpMirrorPrivate,
		DestClusterID:  "", // worker ResolvePreferredCluster 后回写
		AppID:          req.GetAppId(),
		FromUID:        req.GetFrom(),
		TargetUID:      req.GetTarget(),
		Mid:            req.GetMid(),
		Seq:            req.GetSeq(),
		FrameType:      req.GetFrameType(),
		Payload:        payload,
		NeedStore:      true,
	}); err != nil {
		c.log.Error("EnqueueMirrorPrivate", "appId", req.GetAppId(), "mid", req.GetMid(),
			"from", req.GetFrom(), "target", req.GetTarget(), "key", key, "err", err)
		return err
	}
	c.log.Info("EnqueueMirrorPrivate ok", "appId", req.GetAppId(), "mid", req.GetMid(),
		"from", req.GetFrom(), "target", req.GetTarget(), "key", key)
	return nil
}

func (c *Client) ClusterID() string {
	if c == nil {
		return ""
	}
	return c.clusterID
}

func (c *Client) deliver(ctx context.Context, dest string, req *pb.DeliverPrivateReq) error {
	addr, ok := c.bridgeAddr(dest)
	if !ok {
		c.refreshClusters()
		addr, ok = c.bridgeAddr(dest)
		if !ok {
			return fmt.Errorf("unknown dest cluster %s", dest)
		}
	}
	dialCtx, cancel := context.WithTimeout(ctx, c.rpcTimeout)
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

func (c *Client) syncState(ctx context.Context, dest string, req *pb.SyncStateReq) error {
	addr, ok := c.bridgeAddr(dest)
	if !ok {
		c.refreshClusters()
		addr, ok = c.bridgeAddr(dest)
		if !ok {
			return fmt.Errorf("unknown dest cluster %s", dest)
		}
	}
	dialCtx, cancel := context.WithTimeout(ctx, c.rpcTimeout)
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
		return fmt.Errorf("sync rejected")
	}
	return nil
}


// ForwardGroup 按成员 Presence 拆分到各远程集群 DeliverGroup
func (c *Client) ForwardGroup(ctx context.Context, appID, from, groupID string, mid int64, frameType string, msg []byte, memberUIDs []string) error {
	if !c.Enabled() || len(memberUIDs) == 0 {
		return nil
	}
	byCluster := map[string][]string{}
	for _, uid := range memberUIDs {
		clusters, err := c.lookupRemoteClusters(ctx, appID, uid)
		if err != nil {
			c.log.Error("ForwardGroup lookup", "uid", uid, "mid", mid, "err", err)
			continue
		}
		for _, dest := range clusters {
			byCluster[dest] = append(byCluster[dest], uid)
		}
	}
	var lastErr error
	for dest, members := range byCluster {
		req := &pb.DeliverGroupReq{
			SourceCluster:  c.clusterID,
			AppId:          appID,
			From:           from,
			GroupId:        groupID,
			Mid:            mid,
			FrameType:      frameType,
			Msg:            msg,
			MemberUids:     members,
			NeedStore:      false,
			IdempotencyKey: fmt.Sprintf("%s:g:%d:%s", c.clusterID, mid, dest),
			Hop:            1,
		}
		addr, ok := c.bridgeAddr(dest)
		if !ok {
			c.refreshClusters()
			addr, ok = c.bridgeAddr(dest)
		}
		if !ok {
			lastErr = fmt.Errorf("unknown dest cluster %s", dest)
			continue
		}
		dialCtx, cancel := context.WithTimeout(ctx, c.rpcTimeout)
		conn, err := grpc.DialContext(dialCtx, addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
		if err != nil {
			cancel()
			lastErr = err
			continue
		}
		cli := pb.NewFederationClient(conn)
		_, err = cli.DeliverGroup(dialCtx, req)
		conn.Close()
		cancel()
		if err != nil {
			c.log.Error("ForwardGroup DeliverGroup", "dest", dest, "mid", mid, "members", len(members), "err", err)
			lastErr = err
		}
	}
	return lastErr
}
