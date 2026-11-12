package service

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	zlog "github.com/rs/zerolog/log"

	"github.com/web3chat/txchat/dtalk/pkg/mysql"
	"github.com/web3chat/txchat/dtalk/service/record/store/config"
	"github.com/web3chat/txchat/dtalk/service/record/store/dao"
	"github.com/web3chat/txchat/dtalk/service/record/store/model"
)

func newBurnGroupTestService(t *testing.T) *Service {
	t.Helper()
	host := getenvDefault("STORE_MYSQL_HOST", "127.0.0.1")
	port := getenvDefault("STORE_MYSQL_PORT", "3306")
	user := getenvDefault("STORE_MYSQL_USER", "root")
	pwd := getenvDefault("STORE_MYSQL_PWD", "123456")
	dbName := getenvDefault("STORE_MYSQL_DB", "dtalk")
	conn, err := mysql.NewMysqlConn(host, port, user, pwd, dbName, "UTF8MB4")
	if err != nil {
		t.Skipf("skip burn group mysql integration: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return &Service{
		log: zlog.Logger,
		dao: dao.NewWithDB(conn),
		cfg: &config.Config{AppId: "dtalk"},
	}
}

func getenvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func TestBurnGroupTwoReceiversLastLeavesContentOrphan(t *testing.T) {
	s := newBurnGroupTestService(t)
	ctx := context.Background()
	mid := time.Now().UnixNano()
	midStr := strconv.FormatInt(mid, 10)
	gid := fmt.Sprintf("g-burn-%d", mid)
	sender := "sender-burn"
	recv1 := "recv1-burn"
	recv2 := "recv2-burn"

	tx, err := s.dao.NewTx()
	if err != nil {
		t.Fatalf("NewTx: %v", err)
	}
	_, _, err = s.dao.AppendGroupMsgContent(tx, &model.MsgContent{
		Mid:        midStr,
		Seq:        "seq-" + midStr,
		SenderId:   sender,
		ReceiverId: gid,
		MsgType:    1,
		Content:    `{"content":"secret"}`,
		CreateTime: uint64(time.Now().UnixMilli()),
		Source:     `{"burn":true}`,
		Reference:  "",
	})
	if err != nil {
		tx.RollBack()
		t.Fatalf("insert content: %v", err)
	}
	_, _, err = s.dao.AppendGroupMsgRelation(tx, []*model.MsgRelation{
		{Mid: midStr, OwnerUid: sender, OtherUid: gid, Type: model.Send, State: model.Received, CreateTime: uint64(time.Now().UnixMilli())},
		{Mid: midStr, OwnerUid: recv1, OtherUid: gid, Type: model.Rev, State: model.UnReceive, CreateTime: uint64(time.Now().UnixMilli())},
		{Mid: midStr, OwnerUid: recv2, OtherUid: gid, Type: model.Rev, State: model.UnReceive, CreateTime: uint64(time.Now().UnixMilli())},
	})
	if err != nil {
		tx.RollBack()
		t.Fatalf("insert relations: %v", err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatalf("commit seed: %v", err)
	}
	t.Cleanup(func() {
		cleanTx, cerr := s.dao.NewTx()
		if cerr != nil {
			return
		}
		_ = s.dao.DeleteGroupRelationsByMid(cleanTx, mid)
		_, _, _ = cleanTx.Exec("DELETE FROM dtalk_group_msg_content WHERE mid=?", mid)
		_ = cleanTx.Commit()
	})

	status, burned, last, senderID, err := s.BurnGroup(ctx, recv1, gid, mid)
	if err != nil {
		t.Fatalf("first confirm: %v", err)
	}
	if status != model.BurnGroupOK || !burned || last || senderID != sender {
		t.Fatalf("first confirm status=%d burned=%v last=%v sender=%s", status, burned, last, senderID)
	}
	rels, err := s.dao.GetGroupRelationsByMid(mid)
	if err != nil {
		t.Fatalf("relations after first: %v", err)
	}
	if len(rels) != 2 {
		t.Fatalf("after first confirm want 2 relations, got %d", len(rels))
	}

	status, burned, last, senderID, err = s.BurnGroup(ctx, recv2, gid, mid)
	if err != nil {
		t.Fatalf("second confirm: %v", err)
	}
	if status != model.BurnGroupOK || !burned || !last || senderID != sender {
		t.Fatalf("second confirm status=%d burned=%v last=%v sender=%s", status, burned, last, senderID)
	}
	rels, err = s.dao.GetGroupRelationsByMid(mid)
	if err != nil {
		t.Fatalf("relations after last: %v", err)
	}
	if len(rels) != 0 {
		t.Fatalf("last confirm should delete remaining relations, got %d", len(rels))
	}
	content, err := s.dao.GetGroupMsgContent(mid)
	if err != nil {
		t.Fatalf("content should remain: %v", err)
	}
	if content.Mid != midStr {
		t.Fatalf("content mid=%s", content.Mid)
	}
	_, err = s.dao.GetSpecifyGroupRecord(mid)
	if err != model.ErrRecordNotFind {
		t.Fatalf("sync join should miss orphan content, err=%v", err)
	}

	status, burned, last, _, err = s.BurnGroup(ctx, recv2, gid, mid)
	if err != nil {
		t.Fatalf("idempotent confirm: %v", err)
	}
	if status != model.BurnGroupOK || burned || last {
		t.Fatalf("idempotent status=%d burned=%v last=%v", status, burned, last)
	}
}

func TestBurnGroupConfirmBeforeContentIdempotent(t *testing.T) {
	s := newBurnGroupTestService(t)
	status, burned, last, senderID, err := s.BurnGroup(context.Background(), "recv", "gid", time.Now().UnixNano())
	if err != nil {
		t.Fatalf("missing content: %v", err)
	}
	if status != model.BurnGroupOK || burned || last || senderID != "" {
		t.Fatalf("missing content status=%d burned=%v last=%v sender=%s", status, burned, last, senderID)
	}
}
