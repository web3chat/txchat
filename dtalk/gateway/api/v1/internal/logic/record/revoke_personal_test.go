package record

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	xproto "github.com/golang/protobuf/proto"

	"github.com/web3chat/txchat/dtalk/gateway/api/v1/internal/model"
	storeModel "github.com/web3chat/txchat/dtalk/service/record/store/model"
	"github.com/web3chat/txchat/imparse/proto"
)

// mockRecordStore 桩实现 recordStore，可配置返回值并记录调用次数。
type mockRecordStore struct {
	mu sync.Mutex

	getRecordFn func(tp proto.Channel, mid int64) (*storeModel.MsgContent, error)
	delRecordFn func(tp proto.Channel, mid int64) error

	getCalls []int64
	delCalls []int64
}

func (m *mockRecordStore) GetRecord(ctx context.Context, tp proto.Channel, mid int64) (*storeModel.MsgContent, error) {
	m.mu.Lock()
	m.getCalls = append(m.getCalls, mid)
	m.mu.Unlock()
	if m.getRecordFn != nil {
		return m.getRecordFn(tp, mid)
	}
	return nil, nil
}

func (m *mockRecordStore) DelRecord(ctx context.Context, tp proto.Channel, mid int64) error {
	m.mu.Lock()
	m.delCalls = append(m.delCalls, mid)
	m.mu.Unlock()
	if m.delRecordFn != nil {
		return m.delRecordFn(tp, mid)
	}
	return nil
}

// mockSignalCaster 桩实现 signalCaster，记录 UniCastSignal 的 target 顺序与 body。
type mockSignalCaster struct {
	mu       sync.Mutex
	castFn   func(target string, body []byte) error
	castLogs []castEntry
}

type castEntry struct {
	target string
	body   []byte
}

func (m *mockSignalCaster) UniCastSignal(ctx context.Context, action proto.SignalType, target string, body []byte) error {
	m.mu.Lock()
	m.castLogs = append(m.castLogs, castEntry{target: target, body: body})
	m.mu.Unlock()
	if m.castFn != nil {
		return m.castFn(target, body)
	}
	return nil
}

func (m *mockSignalCaster) targets() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, len(m.castLogs))
	for i, e := range m.castLogs {
		out[i] = e.target
	}
	return out
}

func (m *mockSignalCaster) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.castLogs)
}

func decodeRevokeSignal(t *testing.T, body []byte) *proto.SignalRevoke {
	t.Helper()
	sig := &proto.SignalRevoke{}
	require.NoError(t, xproto.Unmarshal(body, sig))
	return sig
}

// newTestLogic 构造一个仅持有 mock 依赖的 Logic，svcCtx 留空（RevokePersonal 不直接使用 svcCtx）。
func newTestLogic(store recordStore, caster signalCaster, expire time.Duration) Logic {
	return Logic{
		ctx:          context.Background(),
		store:        store,
		caster:       caster,
		revokeExpire: expire,
	}
}

func TestRevokePersonal_SenderRevokeNotifiesBothSides(t *testing.T) {
	t.Parallel()

	const (
		sender   = "addr_sender"
		receiver = "addr_receiver"
	)
	now := uint64(time.Now().Unix())

	store := &mockRecordStore{
		getRecordFn: func(_ proto.Channel, _ int64) (*storeModel.MsgContent, error) {
			return &storeModel.MsgContent{
				SenderId:   sender,
				ReceiverId: receiver,
				CreateTime: now,
			}, nil
		},
	}
	caster := &mockSignalCaster{}

	l := newTestLogic(store, caster, time.Hour)
	err := l.RevokePersonal(sender, 1001)
	assert.NoError(t, err)

	// 发送者撤回：保持旧顺序 Receiver → Operator(Sender)
	assert.Equal(t, []string{receiver, sender}, caster.targets())
	// DelRecord 被调用一次
	assert.Equal(t, []int64{1001}, store.delCalls)

	// 两次信令 body 应一致，self=true（操作者=发送者）
	require.Len(t, caster.castLogs, 2)
	sig := decodeRevokeSignal(t, caster.castLogs[0].body)
	assert.Equal(t, int64(1001), sig.Mid)
	assert.Equal(t, sender, sig.Operator)
	assert.True(t, sig.Self, "sender revoke should set self=true")
}

func TestRevokePersonal_ReceiverRevokePeerMessage(t *testing.T) {
	t.Parallel()

	const (
		sender   = "addr_sender"
		receiver = "addr_receiver"
	)
	now := uint64(time.Now().Unix())

	store := &mockRecordStore{
		getRecordFn: func(_ proto.Channel, _ int64) (*storeModel.MsgContent, error) {
			return &storeModel.MsgContent{
				SenderId:   sender,
				ReceiverId: receiver,
				CreateTime: now,
			}, nil
		},
	}
	caster := &mockSignalCaster{}

	l := newTestLogic(store, caster, time.Hour)
	err := l.RevokePersonal(receiver, 2002)
	assert.NoError(t, err)

	// 接收者撤销：仍通知双方，self=false
	assert.Equal(t, []string{sender, receiver}, caster.targets())
	require.Len(t, caster.castLogs, 2)
	sig := decodeRevokeSignal(t, caster.castLogs[0].body)
	assert.Equal(t, receiver, sig.Operator)
	assert.False(t, sig.Self, "receiver revoke should set self=false")
}

func TestRevokePersonal_SelfChatSenderRevokeNotifiesTwice(t *testing.T) {
	t.Parallel()

	const self = "addr_self"
	now := uint64(time.Now().Unix())

	store := &mockRecordStore{
		getRecordFn: func(_ proto.Channel, _ int64) (*storeModel.MsgContent, error) {
			return &storeModel.MsgContent{
				SenderId:   self,
				ReceiverId: self,
				CreateTime: now,
			}, nil
		},
	}
	caster := &mockSignalCaster{}

	l := newTestLogic(store, caster, time.Hour)
	err := l.RevokePersonal(self, 3003)
	assert.NoError(t, err)

	// 自聊发送者撤回：与旧逻辑一致，同一地址下发两次
	assert.Equal(t, []string{self, self}, caster.targets())
	assert.Equal(t, 2, caster.count())
	assert.Equal(t, []int64{3003}, store.delCalls)
}

func TestRevokePersonal_OutsiderDenied(t *testing.T) {
	t.Parallel()

	const (
		sender   = "addr_sender"
		receiver = "addr_receiver"
		outsider = "addr_outsider"
	)
	now := uint64(time.Now().Unix())

	store := &mockRecordStore{
		getRecordFn: func(_ proto.Channel, _ int64) (*storeModel.MsgContent, error) {
			return &storeModel.MsgContent{
				SenderId:   sender,
				ReceiverId: receiver,
				CreateTime: now,
			}, nil
		},
	}
	caster := &mockSignalCaster{}

	l := newTestLogic(store, caster, time.Hour)
	err := l.RevokePersonal(outsider, 4004)
	assert.ErrorIs(t, err, model.ErrPermission)

	// 权限拒绝：不应下发信令，不应删除记录
	assert.Equal(t, 0, caster.count())
	assert.Empty(t, store.delCalls)
}

func TestRevokePersonal_ExpiredDenied(t *testing.T) {
	t.Parallel()

	const (
		sender   = "addr_sender"
		receiver = "addr_receiver"
	)
	expired := uint64(time.Now().Add(-2 * time.Hour).Unix())

	store := &mockRecordStore{
		getRecordFn: func(_ proto.Channel, _ int64) (*storeModel.MsgContent, error) {
			return &storeModel.MsgContent{
				SenderId:   sender,
				ReceiverId: receiver,
				CreateTime: expired,
			}, nil
		},
	}
	caster := &mockSignalCaster{}

	l := newTestLogic(store, caster, time.Hour)
	err := l.RevokePersonal(receiver, 5005)
	assert.ErrorIs(t, err, model.ErrPermission)
	assert.Equal(t, 0, caster.count())
	assert.Empty(t, store.delCalls)
}

func TestRevokePersonal_GetRecordErrorPropagates(t *testing.T) {
	t.Parallel()

	storeErr := errors.New("store rpc timeout")
	store := &mockRecordStore{
		getRecordFn: func(_ proto.Channel, _ int64) (*storeModel.MsgContent, error) {
			return nil, storeErr
		},
	}
	caster := &mockSignalCaster{}

	l := newTestLogic(store, caster, time.Hour)
	err := l.RevokePersonal("anyone", 6006)
	assert.ErrorIs(t, err, storeErr)
	assert.Equal(t, 0, caster.count())
	assert.Empty(t, store.delCalls)
}

func TestRevokePersonal_CastFirstErrorAbortsBeforeDel(t *testing.T) {
	t.Parallel()

	const (
		sender   = "addr_sender"
		receiver = "addr_receiver"
	)
	now := uint64(time.Now().Unix())
	castErr := errors.New("answer rpc error")

	store := &mockRecordStore{
		getRecordFn: func(_ proto.Channel, _ int64) (*storeModel.MsgContent, error) {
			return &storeModel.MsgContent{
				SenderId:   sender,
				ReceiverId: receiver,
				CreateTime: now,
			}, nil
		},
	}
	caster := &mockSignalCaster{
		castFn: func(_ string, _ []byte) error {
			return castErr
		},
	}

	l := newTestLogic(store, caster, time.Hour)
	err := l.RevokePersonal(sender, 7007)
	assert.ErrorIs(t, err, castErr)

	// 发送者撤回时第一次 cast 目标是 receiver，失败应中止
	assert.Equal(t, []string{receiver}, caster.targets())
	assert.Equal(t, 1, caster.count())
	assert.Empty(t, store.delCalls)
}

func TestRevokePersonal_DelRecordErrorPropagatesAfterFirstCast(t *testing.T) {
	t.Parallel()

	const (
		sender   = "addr_sender"
		receiver = "addr_receiver"
	)
	now := uint64(time.Now().Unix())
	delErr := errors.New("del record failed")

	store := &mockRecordStore{
		getRecordFn: func(_ proto.Channel, _ int64) (*storeModel.MsgContent, error) {
			return &storeModel.MsgContent{
				SenderId:   sender,
				ReceiverId: receiver,
				CreateTime: now,
			}, nil
		},
		delRecordFn: func(_ proto.Channel, _ int64) error {
			return delErr
		},
	}
	caster := &mockSignalCaster{}

	l := newTestLogic(store, caster, time.Hour)
	err := l.RevokePersonal(sender, 8008)
	assert.ErrorIs(t, err, delErr)

	// 发送者撤回：已向 receiver 发过一次信令，DelRecord 失败后不应再向 operator 发
	assert.Equal(t, []string{receiver}, caster.targets())
}

func TestRevokePersonal_CastSecondSignalErrorPropagates(t *testing.T) {
	t.Parallel()

	const (
		sender   = "addr_sender"
		receiver = "addr_receiver"
	)
	now := uint64(time.Now().Unix())
	castErr := errors.New("answer rpc error on operator")

	store := &mockRecordStore{
		getRecordFn: func(_ proto.Channel, _ int64) (*storeModel.MsgContent, error) {
			return &storeModel.MsgContent{
				SenderId:   sender,
				ReceiverId: receiver,
				CreateTime: now,
			}, nil
		},
	}
	caster := &mockSignalCaster{
		castFn: func(target string, _ []byte) error {
			// 发送者撤回第二次 cast 目标是 Operator(Sender)
			if target == sender {
				return castErr
			}
			return nil
		},
	}

	l := newTestLogic(store, caster, time.Hour)
	err := l.RevokePersonal(sender, 9009)
	assert.ErrorIs(t, err, castErr)
	// receiver 已通知，operator 通知失败
	assert.Equal(t, []string{receiver, sender}, caster.targets())
	assert.Equal(t, []int64{9009}, store.delCalls)
}
