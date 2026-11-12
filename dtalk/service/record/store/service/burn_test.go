package service

import (
	"testing"

	"github.com/web3chat/txchat/dtalk/service/record/store/model"
)

func TestClassifyBurnGroupOutcome(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		contentFound bool
		isBurn       bool
		receiverID   string
		targetID     string
		hasRev       bool
		hasSend      bool
		wantStatus   int32
		wantDelete   bool
	}{
		{name: "content missing", contentFound: false, wantStatus: model.BurnGroupOK},
		{name: "not burn", contentFound: true, isBurn: false, receiverID: "g1", targetID: "g1", hasRev: true, wantStatus: model.BurnGroupNotBurn},
		{name: "target mismatch", contentFound: true, isBurn: true, receiverID: "g1", targetID: "g2", hasRev: true, wantStatus: model.BurnGroupDenied},
		{name: "has receiver row", contentFound: true, isBurn: true, receiverID: "g1", targetID: "g1", hasRev: true, wantStatus: model.BurnGroupOK, wantDelete: true},
		{name: "no rev no send", contentFound: true, isBurn: true, receiverID: "g1", targetID: "g1", wantStatus: model.BurnGroupOK},
		{name: "sender only", contentFound: true, isBurn: true, receiverID: "g1", targetID: "g1", hasSend: true, wantStatus: model.BurnGroupDenied},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			status, shouldDelete := classifyBurnGroupOutcome(tt.contentFound, tt.isBurn, tt.receiverID, tt.targetID, tt.hasRev, tt.hasSend)
			if status != tt.wantStatus {
				t.Fatalf("status=%d want=%d", status, tt.wantStatus)
			}
			if shouldDelete != tt.wantDelete {
				t.Fatalf("shouldDelete=%v want=%v", shouldDelete, tt.wantDelete)
			}
		})
	}
}

func TestIsLastGroupReceiver(t *testing.T) {
	t.Parallel()
	rels := []*model.MsgRelation{
		{OwnerUid: "r1", Type: model.Rev},
	}
	if !isLastGroupReceiver("r1", rels) {
		t.Fatal("single receiver should be last")
	}
	rels = append(rels, &model.MsgRelation{OwnerUid: "r2", Type: model.Rev})
	if isLastGroupReceiver("r1", rels) {
		t.Fatal("two receivers should not be last")
	}
}

func TestIsBurnReceiver(t *testing.T) {
	t.Parallel()

	rels := []*model.MsgRelation{
		{OwnerUid: "sender", OtherUid: "recv", Type: model.Send},
		{OwnerUid: "recv", OtherUid: "sender", Type: model.Rev},
	}

	if !isBurnReceiver(rels, "recv", "sender") {
		t.Fatal("receiver should be allowed")
	}
	if isBurnReceiver(rels, "sender", "recv") {
		t.Fatal("sender should be denied")
	}
	if isBurnReceiver(rels, "recv", "other") {
		t.Fatal("wrong session should be denied")
	}
	if isBurnReceiver(nil, "recv", "sender") {
		t.Fatal("empty relations should be denied")
	}
}
