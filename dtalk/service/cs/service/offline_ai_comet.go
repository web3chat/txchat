package service

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/inconshreveable/log15"
	"google.golang.org/protobuf/proto"

	comet "github.com/web3chat/txchat/im/api/comet/grpc"
	"github.com/web3chat/txchat/dtalk/service/cs/config"
	xproto "github.com/web3chat/txchat/imparse/proto"
)

func normalizeCometWS(raw string) (string, error) {
	u := strings.TrimSpace(raw)
	if u == "" {
		return "", fmt.Errorf("CometURL 为空")
	}
	if strings.HasPrefix(u, "http://") {
		u = "ws://" + strings.TrimPrefix(u, "http://")
	} else if strings.HasPrefix(u, "https://") {
		u = "wss://" + strings.TrimPrefix(u, "https://")
	} else if !strings.HasPrefix(u, "ws://") && !strings.HasPrefix(u, "wss://") {
		u = "ws://" + u
	}
	if !strings.Contains(u, "/sub") {
		if strings.HasSuffix(u, "/") {
			u = u + "sub"
		} else {
			u = u + "/sub"
		}
	}
	if _, err := url.Parse(u); err != nil {
		return "", err
	}
	return u, nil
}

func dialCometAndAuth(ctx context.Context, log log15.Logger, oaicfg *config.OfflineAIConfig, appID, fzmToken string, seq *atomic.Int32) (*websocket.Conn, error) {
	wsURL, err := normalizeCometWS(oaicfg.CometURL)
	if err != nil {
		return nil, err
	}

	devUUID := strings.TrimSpace(oaicfg.DeviceUUID)
	if devUUID == "" {
		devUUID = uuid.New().String()
	}
	devName := strings.TrimSpace(oaicfg.DeviceName)
	if devName == "" {
		devName = "CS-Offline-AI"
	}
	extLogin := &xproto.Login{
		Device:     xproto.Device_Web,
		Uuid:       devUUID,
		DeviceName: devName,
		ConnType:   xproto.Login_Connect,
	}
	extBytes, err := proto.Marshal(extLogin)
	if err != nil {
		return nil, err
	}

	authBody, err := proto.Marshal(&comet.AuthMsg{
		AppId: appID,
		Token: fzmToken,
		Ext:   extBytes,
	})
	if err != nil {
		return nil, err
	}

	d := websocket.Dialer{HandshakeTimeout: 15 * time.Second}
	conn, _, err := d.DialContext(ctx, wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("websocket dial %s: %w", wsURL, err)
	}

	authProto := &comet.Proto{
		Ver:  1,
		Op:   int32(comet.Op_Auth),
		Seq:  0,
		Ack:  0,
		Body: authBody,
	}
	if err := authProto.WriteWebsocket2(conn); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("write auth: %w", err)
	}

	reply := new(comet.Proto)
	if err := reply.ReadWebsocket2(conn); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("read auth reply: %w", err)
	}
	if reply.Op != int32(comet.Op_AuthReply) {
		_ = conn.Close()
		return nil, fmt.Errorf("unexpected op after auth: %d", reply.Op)
	}
	if len(reply.Body) > 0 {
		_ = conn.Close()
		return nil, fmt.Errorf("auth failed body=%s", string(reply.Body))
	}
	seq.Store(0)
	log.Info("离线 AI：Comet 认证成功", "ws", wsURL)
	return conn, nil
}

func runHeartbeat(ctx context.Context, seq *atomic.Int32, sec int, log log15.Logger, send func(*comet.Proto) error) {
	if sec <= 0 {
		sec = 28
	}
	t := time.NewTicker(time.Duration(sec) * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			p := &comet.Proto{
				Ver: 1,
				Op:  int32(comet.Op_Heartbeat),
				Seq: seq.Add(1),
				Ack: 0,
			}
			if err := send(p); err != nil {
				log.Warn("离线 AI：心跳发送失败", "err", err)
				return
			}
		}
	}
}
