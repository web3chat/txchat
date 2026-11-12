package record

import (
	"context"
	"sync"
	"testing"

	"github.com/golang/protobuf/proto"

	"github.com/web3chat/txchat/dtalk/gateway/api/v1/internal/model"
	store "github.com/web3chat/txchat/dtalk/service/record/store/api"
	storeModel "github.com/web3chat/txchat/dtalk/service/record/store/model"
	xproto "github.com/web3chat/txchat/imparse/proto"
)

type mockBurnStore struct {
	mu           sync.Mutex
	privateFn    func(operator, targetID string, mid int64) (*store.BurnPrivateReply, error)
	groupFn      func(operator, targetID string, mid int64) (*store.BurnGroupReply, error)
	privateCalls int
	groupCalls   int
}

func (m *mockBurnStore) BurnPrivate(ctx context.Context, operator, targetID string, mid int64) (*store.BurnPrivateReply, error) {
	m.mu.Lock()
	m.privateCalls++
	m.mu.Unlock()
	if m.privateFn != nil {
		return m.privateFn(operator, targetID, mid)
	}
	return &store.BurnPrivateReply{Status: storeModel.BurnPrivateOK, Mid: mid}, nil
}

func (m *mockBurnStore) BurnGroup(ctx context.Context, operator, targetID string, mid int64) (*store.BurnGroupReply, error) {
	m.mu.Lock()
	m.groupCalls++
	m.mu.Unlock()
	if m.groupFn != nil {
		return m.groupFn(operator, targetID, mid)
	}
	return &store.BurnGroupReply{Status: storeModel.BurnGroupOK, Mid: mid}, nil
}

func (m *mockBurnStore) counts() (private, group int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.privateCalls, m.groupCalls
}

func TestBurnDedupKey(t *testing.T) {
	t.Parallel()
	if got := burnDedupKey("uid-1", 99); got != "uid-1:99" {
		t.Fatalf("burnDedupKey = %s", got)
	}
}

func TestMarshalRevokeSignalBurned(t *testing.T) {
	t.Parallel()

	body, err := marshalRevokeSignal("operator", 123456, false, xproto.RevokeReason_burned)
	if err != nil {
		t.Fatalf("marshalRevokeSignal: %v", err)
	}
	var action xproto.SignalRevoke
	if err = proto.Unmarshal(body, &action); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if action.GetMid() != 123456 {
		t.Fatalf("mid=%d", action.GetMid())
	}
	if action.GetOperator() != "operator" {
		t.Fatalf("operator=%s", action.GetOperator())
	}
	if action.GetSelf() {
		t.Fatal("self should be false for receiver confirm")
	}
	if action.GetReason() != xproto.RevokeReason_burned {
		t.Fatalf("reason=%v", action.GetReason())
	}
}

func TestBurnConfirmDefaultTypeCallsPrivate(t *testing.T) {
	resetBurnDedup()
	st := &mockBurnStore{}
	caster := &mockSignalCaster{}
	l := Logic{ctx: context.Background(), burner: st, caster: caster}

	resp, err := l.BurnConfirm("recv", "peer", 11, model.Private)
	if err != nil {
		t.Fatalf("BurnConfirm: %v", err)
	}
	if resp.Burned {
		t.Fatal("empty private reply should be burned=false")
	}
	p, g := st.counts()
	if p != 1 || g != 0 {
		t.Fatalf("calls private=%d group=%d", p, g)
	}
}

func TestBurnConfirmGroupLastUnicastSender(t *testing.T) {
	resetBurnDedup()
	st := &mockBurnStore{
		groupFn: func(operator, targetID string, mid int64) (*store.BurnGroupReply, error) {
			return &store.BurnGroupReply{
				Status:   storeModel.BurnGroupOK,
				Burned:   true,
				Last:     true,
				SenderId: "sender",
				Mid:      mid,
			}, nil
		},
	}
	caster := &mockSignalCaster{}
	l := Logic{ctx: context.Background(), burner: st, caster: caster}

	resp, err := l.BurnConfirm("recv", "gid-1", 22, model.Group)
	if err != nil {
		t.Fatalf("BurnConfirm: %v", err)
	}
	if !resp.Burned {
		t.Fatal("want burned=true")
	}
	p, g := st.counts()
	if p != 0 || g != 1 {
		t.Fatalf("calls private=%d group=%d", p, g)
	}
	got := caster.targets()
	if len(got) != 2 || got[0] != "recv" || got[1] != "sender" {
		t.Fatalf("targets=%v want [recv sender]", got)
	}
}

func TestBurnConfirmGroupNotLastOnlyConfirmer(t *testing.T) {
	resetBurnDedup()
	st := &mockBurnStore{
		groupFn: func(operator, targetID string, mid int64) (*store.BurnGroupReply, error) {
			return &store.BurnGroupReply{
				Status:   storeModel.BurnGroupOK,
				Burned:   true,
				Last:     false,
				SenderId: "sender",
				Mid:      mid,
			}, nil
		},
	}
	caster := &mockSignalCaster{}
	l := Logic{ctx: context.Background(), burner: st, caster: caster}

	if _, err := l.BurnConfirm("recv", "gid-1", 23, model.Group); err != nil {
		t.Fatalf("BurnConfirm: %v", err)
	}
	got := caster.targets()
	if len(got) != 1 || got[0] != "recv" {
		t.Fatalf("targets=%v want [recv]", got)
	}
}

func TestBurnConfirmDedupOnlyAfterBurnedTrue(t *testing.T) {
	resetBurnDedup()
	st := &mockBurnStore{
		privateFn: func(operator, targetID string, mid int64) (*store.BurnPrivateReply, error) {
			return &store.BurnPrivateReply{Status: storeModel.BurnPrivateOK, Burned: false, Mid: mid}, nil
		},
	}
	l := Logic{ctx: context.Background(), burner: st, caster: &mockSignalCaster{}}
	if _, err := l.BurnConfirm("recv", "peer", 31, model.Private); err != nil {
		t.Fatalf("first: %v", err)
	}
	if _, err := l.BurnConfirm("recv", "peer", 31, model.Private); err != nil {
		t.Fatalf("second: %v", err)
	}
	p, _ := st.counts()
	if p != 2 {
		t.Fatalf("burned=false should not write dedup window, privateCalls=%d", p)
	}

	resetBurnDedup()
	st = &mockBurnStore{
		privateFn: func(operator, targetID string, mid int64) (*store.BurnPrivateReply, error) {
			return &store.BurnPrivateReply{
				Status:     storeModel.BurnPrivateOK,
				Burned:     true,
				SenderId:   "sender",
				ReceiverId: "recv",
				Mid:        mid,
			}, nil
		},
	}
	caster := &mockSignalCaster{}
	l = Logic{ctx: context.Background(), burner: st, caster: caster}
	if _, err := l.BurnConfirm("recv", "peer", 32, model.Private); err != nil {
		t.Fatalf("burned true: %v", err)
	}
	if _, err := l.BurnConfirm("recv", "peer", 32, model.Private); err != nil {
		t.Fatalf("dedup: %v", err)
	}
	p, _ = st.counts()
	if p != 1 {
		t.Fatalf("burned=true window should skip store, privateCalls=%d", p)
	}
	if len(caster.targets()) != 2 {
		t.Fatalf("dedup must not resend signal, targets=%v", caster.targets())
	}
}

func TestBurnConfirmInvalidType(t *testing.T) {
	resetBurnDedup()
	st := &mockBurnStore{}
	l := Logic{ctx: context.Background(), burner: st, caster: &mockSignalCaster{}}
	if _, err := l.BurnConfirm("recv", "peer", 41, 2); err == nil {
		t.Fatal("type=2 should be params error")
	}
	p, g := st.counts()
	if p != 0 || g != 0 {
		t.Fatalf("invalid type must not call store, private=%d group=%d", p, g)
	}
}
