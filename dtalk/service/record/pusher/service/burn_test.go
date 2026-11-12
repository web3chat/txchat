package service

import (
	"testing"

	"github.com/golang/protobuf/proto"
	record "github.com/web3chat/txchat/dtalk/service/record/proto"
	comet "github.com/web3chat/txchat/im/api/comet/grpc"
	xproto "github.com/web3chat/txchat/imparse/proto"
)

func TestShouldForwardGroup(t *testing.T) {
	t.Parallel()
	if shouldForwardGroup(true, true) {
		t.Fatal("burn should not ForwardGroup even if federation enabled")
	}
	if shouldForwardGroup(true, false) {
		t.Fatal("burn should not ForwardGroup when federation disabled")
	}
	if shouldForwardGroup(false, false) {
		t.Fatal("non-burn should not ForwardGroup when federation disabled")
	}
	if !shouldForwardGroup(false, true) {
		t.Fatal("non-burn with federation enabled should ForwardGroup")
	}
}

func TestIsBurnPushMsg(t *testing.T) {
	t.Parallel()

	if isBurnPushMsg(&record.PushMsg{}) {
		t.Fatal("empty msg should be non-burn")
	}
	if isBurnPushMsg(&record.PushMsg{Msg: []byte("not-proto")}) {
		t.Fatal("invalid proto should be non-burn")
	}
	if isBurnPushMsg(&record.PushMsg{Msg: mustEncodePush(t, false)}) {
		t.Fatal("source.burn=false should be non-burn")
	}
	if !isBurnPushMsg(&record.PushMsg{Msg: mustEncodePush(t, true)}) {
		t.Fatal("source.burn=true should skip federation")
	}
}

func mustEncodePush(t *testing.T, burn bool) []byte {
	t.Helper()
	commonBytes, err := proto.Marshal(&xproto.Common{
		Source: &xproto.Source{Burn: burn},
	})
	if err != nil {
		t.Fatalf("marshal common: %v", err)
	}
	bizBytes, err := proto.Marshal(&xproto.Proto{
		EventType: xproto.Proto_common,
		Body:      commonBytes,
	})
	if err != nil {
		t.Fatalf("marshal biz proto: %v", err)
	}
	msg, err := proto.Marshal(&comet.Proto{Body: bizBytes})
	if err != nil {
		t.Fatalf("marshal comet proto: %v", err)
	}
	return msg
}
