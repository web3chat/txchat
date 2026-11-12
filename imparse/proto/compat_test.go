package proto

import (
	"testing"

	goproto "google.golang.org/protobuf/proto"
)

func TestCompatBurnRoundTrip(t *testing.T) {
	in := &Common{
		ChannelType: Channel_ToUser,
		MsgType:     MsgType_Text,
		Source:      &Source{Burn: true},
	}
	raw, err := goproto.Marshal(in)
	if err != nil {
		t.Fatalf("marshal common: %v", err)
	}
	var out Common
	if err := goproto.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal common: %v", err)
	}
	if !out.GetSource().GetBurn() {
		t.Fatal("expected source.burn=true after round trip")
	}

	revoke := &SignalRevoke{Mid: 1, Reason: RevokeReason_burned}
	raw, err = goproto.Marshal(revoke)
	if err != nil {
		t.Fatalf("marshal revoke: %v", err)
	}
	var got SignalRevoke
	if err := goproto.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal revoke: %v", err)
	}
	if got.GetReason() != RevokeReason_burned {
		t.Fatalf("reason=%v, want burned", got.GetReason())
	}

	_ = Device_Android
	_ = Login_Reconnect
	_ = SignalType_Revoke
}
