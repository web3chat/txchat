package dao

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/web3chat/txchat/dtalk/pkg/mysql"
	"github.com/web3chat/txchat/dtalk/pkg/util"
	conf "github.com/web3chat/txchat/dtalk/service/federation/config"
)

const (
	StatusPending = 0
	StatusSending = 1
	StatusDone    = 2
	StatusDead    = 3

	OpDeliverPrivate = "deliver_private"
	OpMirrorPrivate  = "mirror_private"
	OpSyncState      = "sync_state"
	OpDeliverGroup   = "deliver_group"
)

var (
	ErrIdempotencyConflict = errors.New("IDEMPOTENCY_CONFLICT")
	ErrMidCollision        = errors.New("MID_COLLISION")
)

type OutboxRow struct {
	ID             int64
	IdempotencyKey string
	OpType         string
	DestClusterID  string
	AppID          string
	FromUID        string
	TargetUID      string
	Mid            int64
	Seq            string
	FrameType      string
	Payload        []byte
	NeedStore      bool
	Status         int
	RetryCount     int
	NextRetryAt    int64
	LastError      string
	LeaseOwner     string
	LeaseExpireAt  int64
}

type MirrorPrivateInput struct {
	IdempotencyKey string
	SourceCluster  string
	AppID          string
	From           string
	Target         string
	Mid            int64
	Seq            string
	MsgType        uint32
	Content        string
	Source         string
	Reference      string
	CreateTime     uint64
	ContentHash    string
}

type Dao struct {
	log log15.Logger
	db  *mysql.MysqlConn
}

func New(c *conf.Config) *Dao {
	db, err := mysql.NewMysqlConn(c.MySQL.Host, c.MySQL.Port, c.MySQL.User, c.MySQL.Pwd, c.MySQL.DbName)
	if err != nil {
		panic(fmt.Sprintf("federation dao mysql: %v", err))
	}
	return &Dao{log: log15.New("module", "federation/dao"), db: db}
}

func (d *Dao) Close() {
	if d.db != nil {
		_ = d.db.Close()
	}
}

func (d *Dao) InsertOutbox(row *OutboxRow) error {
	now := time.Now().UnixMilli()
	needStore := 0
	if row.NeedStore {
		needStore = 1
	}
	opType := row.OpType
	if opType == "" {
		opType = OpDeliverPrivate
	}
	payloadB64 := base64.StdEncoding.EncodeToString(row.Payload)
	_, _, err := d.db.Exec(`INSERT INTO dtalk_federation_outbox
		(idempotency_key,op_type,dest_cluster_id,app_id,from_uid,target_uid,mid,seq,frame_type,payload,need_store,status,retry_count,next_retry_at,last_error,lease_owner,lease_expire_at,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON DUPLICATE KEY UPDATE updated_at=VALUES(updated_at)`,
		row.IdempotencyKey, opType, row.DestClusterID, row.AppID, row.FromUID, row.TargetUID, row.Mid, row.Seq, row.FrameType, []byte(payloadB64), needStore,
		StatusPending, 0, now, "", "", int64(0), now, now)
	if err != nil {
		d.log.Error("InsertOutbox", "key", row.IdempotencyKey, "op", opType, "mid", row.Mid, "dest", row.DestClusterID, "err", err)
	}
	return err
}

// ClaimPending 短事务租约领取：pending 或 lease 过期的 sending。
func (d *Dao) ClaimPending(limit int, now int64, leaseOwner string, leaseDurationMs int64) ([]*OutboxRow, error) {
	if limit <= 0 {
		limit = 50
	}
	if leaseOwner == "" {
		leaseOwner = "worker"
	}
	if leaseDurationMs <= 0 {
		leaseDurationMs = 30_000
	}
	tx, err := d.db.NewTx()
	if err != nil {
		d.log.Error("ClaimPending begin", "err", err)
		return nil, err
	}
	rows, err := tx.Query(`SELECT id FROM dtalk_federation_outbox
		WHERE (status=? OR (status=? AND lease_expire_at<?)) AND next_retry_at<=?
		ORDER BY id ASC LIMIT ? FOR UPDATE SKIP LOCKED`,
		StatusPending, StatusSending, now, now, limit)
	if err != nil {
		tx.RollBack()
		// 兼容未升级 schema：回退旧领取逻辑
		if strings.Contains(err.Error(), "lease_expire_at") || strings.Contains(err.Error(), "Unknown column") || strings.Contains(err.Error(), "SKIP LOCKED") {
			return d.claimPendingLegacy(limit, now)
		}
		d.log.Error("ClaimPending select", "err", err)
		return nil, err
	}
	ids := make([]int64, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, util.ToInt64(r["id"]))
	}
	if len(ids) == 0 {
		_ = tx.Commit()
		return nil, nil
	}
	expireAt := now + leaseDurationMs
	for _, id := range ids {
		_, _, err = tx.Exec(`UPDATE dtalk_federation_outbox
			SET status=?, lease_owner=?, lease_expire_at=?, updated_at=?
			WHERE id=? AND (status=? OR (status=? AND lease_expire_at<?))`,
			StatusSending, leaseOwner, expireAt, now, id, StatusPending, StatusSending, now)
		if err != nil {
			tx.RollBack()
			d.log.Error("ClaimPending update", "id", id, "err", err)
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		d.log.Error("ClaimPending commit", "err", err)
		return nil, err
	}
	return d.loadOutboxByIDs(ids)
}

func (d *Dao) claimPendingLegacy(limit int, now int64) ([]*OutboxRow, error) {
	rows, err := d.db.Query(`SELECT id,idempotency_key,dest_cluster_id,app_id,from_uid,target_uid,mid,seq,frame_type,payload,need_store,status,retry_count,next_retry_at,last_error
		FROM dtalk_federation_outbox WHERE status IN (?,?) AND next_retry_at<=? ORDER BY id ASC LIMIT ?`,
		StatusPending, StatusSending, now, limit)
	if err != nil {
		d.log.Error("ClaimPending legacy", "err", err)
		return nil, err
	}
	out := make([]*OutboxRow, 0, len(rows))
	for _, r := range rows {
		id := util.ToInt64(r["id"])
		_, _, _ = d.db.Exec(`UPDATE dtalk_federation_outbox SET status=?, updated_at=? WHERE id=? AND status IN (?,?)`,
			StatusSending, now, id, StatusPending, StatusSending)
		out = append(out, mapOutboxRow(r, OpDeliverPrivate))
	}
	return out, nil
}

func (d *Dao) loadOutboxByIDs(ids []int64) ([]*OutboxRow, error) {
	out := make([]*OutboxRow, 0, len(ids))
	for _, id := range ids {
		rows, err := d.db.Query(`SELECT id,idempotency_key,op_type,dest_cluster_id,app_id,from_uid,target_uid,mid,seq,frame_type,payload,need_store,status,retry_count,next_retry_at,last_error,lease_owner,lease_expire_at
			FROM dtalk_federation_outbox WHERE id=? LIMIT 1`, id)
		if err != nil {
			// op_type/lease 列可能尚未迁移
			rows, err = d.db.Query(`SELECT id,idempotency_key,dest_cluster_id,app_id,from_uid,target_uid,mid,seq,frame_type,payload,need_store,status,retry_count,next_retry_at,last_error
				FROM dtalk_federation_outbox WHERE id=? LIMIT 1`, id)
			if err != nil {
				return nil, err
			}
			if len(rows) == 0 {
				continue
			}
			out = append(out, mapOutboxRow(rows[0], OpDeliverPrivate))
			continue
		}
		if len(rows) == 0 {
			continue
		}
		op := rows[0]["op_type"]
		if op == "" {
			op = OpDeliverPrivate
		}
		row := mapOutboxRow(rows[0], op)
		row.LeaseOwner = rows[0]["lease_owner"]
		row.LeaseExpireAt = util.ToInt64(rows[0]["lease_expire_at"])
		out = append(out, row)
	}
	return out, nil
}

func mapOutboxRow(r map[string]string, defaultOp string) *OutboxRow {
	need := util.ToInt64(r["need_store"]) == 1
	payload, decErr := base64.StdEncoding.DecodeString(r["payload"])
	if decErr != nil {
		payload = []byte(r["payload"])
	}
	op := r["op_type"]
	if op == "" {
		op = defaultOp
	}
	return &OutboxRow{
		ID:             util.ToInt64(r["id"]),
		IdempotencyKey: r["idempotency_key"],
		OpType:         op,
		DestClusterID:  r["dest_cluster_id"],
		AppID:          r["app_id"],
		FromUID:        r["from_uid"],
		TargetUID:      r["target_uid"],
		Mid:            util.ToInt64(r["mid"]),
		Seq:            r["seq"],
		FrameType:      r["frame_type"],
		Payload:        payload,
		NeedStore:      need,
		Status:         int(util.ToInt64(r["status"])),
		RetryCount:     int(util.ToInt64(r["retry_count"])),
		NextRetryAt:    util.ToInt64(r["next_retry_at"]),
		LastError:      r["last_error"],
	}
}

func (d *Dao) UpdateOutboxDest(id int64, destClusterID string) error {
	now := time.Now().UnixMilli()
	_, _, err := d.db.Exec(`UPDATE dtalk_federation_outbox SET dest_cluster_id=?, updated_at=? WHERE id=?`,
		destClusterID, now, id)
	if err != nil {
		d.log.Error("UpdateOutboxDest", "id", id, "dest", destClusterID, "err", err)
	}
	return err
}

func (d *Dao) MarkOutboxDone(id int64, leaseOwner string) error {
	now := time.Now().UnixMilli()
	var err error
	if leaseOwner == "" {
		_, _, err = d.db.Exec(`UPDATE dtalk_federation_outbox SET status=?, updated_at=? WHERE id=?`, StatusDone, now, id)
	} else {
		_, _, err = d.db.Exec(`UPDATE dtalk_federation_outbox SET status=?, lease_expire_at=0, updated_at=? WHERE id=? AND lease_owner=?`,
			StatusDone, now, id, leaseOwner)
	}
	if err != nil {
		d.log.Error("MarkOutboxDone", "id", id, "owner", leaseOwner, "err", err)
	}
	return err
}

func (d *Dao) MarkOutboxRetry(id int64, retryCount int, nextRetryAt int64, lastErr string, dead bool) error {
	now := time.Now().UnixMilli()
	st := StatusPending
	if dead {
		st = StatusDead
	}
	if len(lastErr) > 500 {
		lastErr = lastErr[:500]
	}
	_, _, err := d.db.Exec(`UPDATE dtalk_federation_outbox SET status=?, retry_count=?, next_retry_at=?, last_error=?, lease_owner='', lease_expire_at=0, updated_at=? WHERE id=?`,
		st, retryCount, nextRetryAt, lastErr, now, id)
	if err != nil {
		// 兼容未升级列
		_, _, err = d.db.Exec(`UPDATE dtalk_federation_outbox SET status=?, retry_count=?, next_retry_at=?, last_error=?, updated_at=? WHERE id=?`,
			st, retryCount, nextRetryAt, lastErr, now, id)
	}
	if err != nil {
		d.log.Error("MarkOutboxRetry", "id", id, "retry", retryCount, "dead", dead, "err", err)
	}
	return err
}

func (d *Dao) InsertInbox(idempotencyKey, sourceCluster string, mid int64, appID, from, target, opType, contentHash string, stored bool) (inserted bool, err error) {
	now := time.Now().UnixMilli()
	st := 0
	if stored {
		st = 1
	}
	// stored 为 MySQL 保留字，列名必须加反引号
	_, _, err = d.db.Exec(
		"INSERT INTO dtalk_federation_inbox "+
			"(idempotency_key,source_cluster_id,mid,app_id,from_uid,target_uid,op_type,content_hash,`stored`,created_at) "+
			"VALUES(?,?,?,?,?,?,?,?,?,?)",
		idempotencyKey, sourceCluster, mid, appID, from, target, opType, contentHash, st, now)
	if err != nil {
		if isDup(err) {
			return false, nil
		}
		// 兼容无 content_hash 列
		if strings.Contains(err.Error(), "content_hash") || strings.Contains(err.Error(), "Unknown column") {
			_, _, err = d.db.Exec(
				"INSERT INTO dtalk_federation_inbox "+
					"(idempotency_key,source_cluster_id,mid,app_id,from_uid,target_uid,op_type,`stored`,created_at) "+
					"VALUES(?,?,?,?,?,?,?,?,?)",
				idempotencyKey, sourceCluster, mid, appID, from, target, opType, st, now)
			if err != nil {
				if isDup(err) {
					return false, nil
				}
				d.log.Error("InsertInbox", "key", idempotencyKey, "mid", mid, "err", err)
				return false, err
			}
			return true, nil
		}
		d.log.Error("InsertInbox", "key", idempotencyKey, "mid", mid, "err", err)
		return false, err
	}
	return true, nil
}

func (d *Dao) InboxExists(idempotencyKey string) (bool, error) {
	rows, err := d.db.Query(`SELECT idempotency_key FROM dtalk_federation_inbox WHERE idempotency_key=? LIMIT 1`, idempotencyKey)
	if err != nil {
		return false, err
	}
	return len(rows) > 0, nil
}

func (d *Dao) GetInboxContentHash(idempotencyKey string) (string, bool, error) {
	rows, err := d.db.Query(`SELECT content_hash FROM dtalk_federation_inbox WHERE idempotency_key=? LIMIT 1`, idempotencyKey)
	if err != nil {
		if strings.Contains(err.Error(), "content_hash") || strings.Contains(err.Error(), "Unknown column") {
			exists, e2 := d.InboxExists(idempotencyKey)
			return "", exists, e2
		}
		return "", false, err
	}
	if len(rows) == 0 {
		return "", false, nil
	}
	return rows[0]["content_hash"], true, nil
}

// StoreMirroredPrivate 冲突安全落库：inbox + content + relation 同一事务。
// duplicated=true 表示合法幂等重试。
func (d *Dao) StoreMirroredPrivate(in MirrorPrivateInput) (duplicated bool, err error) {
	if retainExpired(in.Mid, in.CreateTime, time.Now()) {
		d.log.Info("StoreMirroredPrivate skip expired", "mid", in.Mid, "createTime", in.CreateTime)
		return true, nil
	}
	if in.ContentHash == "" {
		in.ContentHash = ContentFingerprint(in.AppID, in.Mid, in.Seq, in.From, in.Target, in.MsgType, in.CreateTime, in.Content, in.Source, in.Reference)
	}
	tx, err := d.db.NewTx()
	if err != nil {
		return false, err
	}
	now := time.Now().UnixMilli()
	// stored 为 MySQL 保留字，列名必须加反引号
	_, _, err = tx.Exec(
		"INSERT INTO dtalk_federation_inbox "+
			"(idempotency_key,source_cluster_id,mid,app_id,from_uid,target_uid,op_type,content_hash,`stored`,created_at) "+
			"VALUES(?,?,?,?,?,?,?,?,?,?)",
		in.IdempotencyKey, in.SourceCluster, in.Mid, in.AppID, in.From, in.Target, OpMirrorPrivate, in.ContentHash, 1, now)
	if err != nil {
		if !isDup(err) {
			tx.RollBack()
			d.log.Error("StoreMirroredPrivate inbox", "key", in.IdempotencyKey, "mid", in.Mid, "err", err)
			return false, err
		}
		tx.RollBack()
		existHash, found, gerr := d.GetInboxContentHash(in.IdempotencyKey)
		if gerr != nil {
			return false, gerr
		}
		if !found {
			return false, fmt.Errorf("inbox duplicate but not found key=%s", in.IdempotencyKey)
		}
		if existHash == "" || existHash == in.ContentHash {
			return true, nil
		}
		d.log.Error("StoreMirroredPrivate idempotency conflict",
			"key", in.IdempotencyKey, "mid", in.Mid, "existHash", existHash, "newHash", in.ContentHash)
		return false, ErrIdempotencyConflict
	}

	midStr := fmt.Sprintf("%d", in.Mid)
	_, _, err = tx.Exec(`INSERT INTO dtalk_msg_content(mid,seq,sender_id,receiver_id,msg_type,content,create_time,source,reference,source_cluster)
		VALUES(?,?,?,?,?,?,?,?,?,?)`,
		midStr, in.Seq, in.From, in.Target, in.MsgType, in.Content, in.CreateTime, in.Source, in.Reference, in.SourceCluster)
	if err != nil {
		if !isDup(err) {
			tx.RollBack()
			d.log.Error("StoreMirroredPrivate content", "mid", in.Mid, "key", in.IdempotencyKey, "err", err)
			return false, err
		}
		same, cerr := d.sameContentFingerprint(tx, midStr, in)
		if cerr != nil {
			tx.RollBack()
			return false, cerr
		}
		if !same {
			tx.RollBack()
			d.log.Error("StoreMirroredPrivate mid collision", "mid", in.Mid, "key", in.IdempotencyKey)
			return false, ErrMidCollision
		}
		// 合法重复：content 已存在且指纹相同，继续确保 relation
	}
	if err = appendRelations(tx, midStr, in.From, in.Target, in.CreateTime); err != nil {
		tx.RollBack()
		d.log.Error("StoreMirroredPrivate relation", "mid", in.Mid, "err", err)
		return false, err
	}
	if err = tx.Commit(); err != nil {
		d.log.Error("StoreMirroredPrivate commit", "mid", in.Mid, "err", err)
		return false, err
	}
	return false, nil
}

func (d *Dao) sameContentFingerprint(tx *mysql.MysqlTx, midStr string, in MirrorPrivateInput) (bool, error) {
	rows, err := tx.Query(`SELECT mid,seq,sender_id,receiver_id,msg_type,content,create_time,source,reference,source_cluster
		FROM dtalk_msg_content WHERE mid=? LIMIT 1`, midStr)
	if err != nil {
		// 兼容无 source_cluster
		rows, err = tx.Query(`SELECT mid,seq,sender_id,receiver_id,msg_type,content,create_time,source,reference
			FROM dtalk_msg_content WHERE mid=? LIMIT 1`, midStr)
		if err != nil {
			return false, err
		}
	}
	if len(rows) == 0 {
		return false, fmt.Errorf("content missing after duplicate mid=%s", midStr)
	}
	r := rows[0]
	existHash := ContentFingerprint(
		in.AppID,
		util.ToInt64(r["mid"]),
		r["seq"],
		r["sender_id"],
		r["receiver_id"],
		uint32(util.ToInt64(r["msg_type"])),
		uint64(util.ToInt64(r["create_time"])),
		r["content"],
		r["source"],
		r["reference"],
	)
	return existHash == in.ContentHash, nil
}

func ContentFingerprint(appID string, mid int64, seq, from, target string, msgType uint32, createTime uint64, content, source, reference string) string {
	raw := fmt.Sprintf("%s|%d|%s|%s|%s|%d|%d|%s|%s|%s",
		appID, mid, seq, from, target, msgType, createTime, content, source, reference)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func (d *Dao) AppendFederatedMsg(mid int64, seq, from, target string, msgType uint32, content, source, reference, sourceCluster string, createTime uint64) error {
	if retainExpired(mid, createTime, time.Now()) {
		d.log.Info("AppendFederatedMsg skip expired", "mid", mid, "createTime", createTime)
		return nil
	}
	midStr := fmt.Sprintf("%d", mid)
	tx, err := d.db.NewTx()
	if err != nil {
		return err
	}
	_, _, err = tx.Exec(`INSERT INTO dtalk_msg_content(mid,seq,sender_id,receiver_id,msg_type,content,create_time,source,reference,source_cluster)
		VALUES(?,?,?,?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE mid=mid`,
		midStr, seq, from, target, msgType, content, createTime, source, reference, sourceCluster)
	if err != nil {
		tx.RollBack()
		d.log.Error("AppendFederatedMsg content with source_cluster failed, fallback", "mid", mid, "err", err)
		tx2, err2 := d.db.NewTx()
		if err2 != nil {
			return err
		}
		_, _, err2 = tx2.Exec(`INSERT INTO dtalk_msg_content(mid,seq,sender_id,receiver_id,msg_type,content,create_time,source,reference)
			VALUES(?,?,?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE mid=mid`,
			midStr, seq, from, target, msgType, content, createTime, source, reference)
		if err2 != nil {
			tx2.RollBack()
			return err2
		}
		if err2 = appendRelations(tx2, midStr, from, target, createTime); err2 != nil {
			tx2.RollBack()
			return err2
		}
		return tx2.Commit()
	}
	if err = appendRelations(tx, midStr, from, target, createTime); err != nil {
		tx.RollBack()
		return err
	}
	return tx.Commit()
}

// appendRelations 写入发送方/接收方各一条 relation。
// 联邦补历史场景下双方均写 state=1（可被 /pri-chat-record 查出）；
// 不可再给接收方写 state=0：客户端常去对端归属集群拉历史，state=0 会被 API 过滤导致「只见自己发出的消息」。
// 幂等重试时：仅把 state=0 提升为 1，不覆盖已撤回（state=2）。
func appendRelations(tx *mysql.MysqlTx, midStr, from, target string, createTime uint64) error {
	const (
		relTypeSend = 0
		relTypeRev  = 1
		relStateOK  = 1 // Received：历史可查
	)
	healState := `ON DUPLICATE KEY UPDATE state=IF(state=0,1,state)`
	_, _, err := tx.Exec(
		`INSERT INTO dtalk_msg_relation(mid,owner_uid,other_uid,type,state,create_time) VALUES(?,?,?,?,?,?) `+healState,
		midStr, from, target, relTypeSend, relStateOK, createTime)
	if err != nil {
		return err
	}
	_, _, err = tx.Exec(
		`INSERT INTO dtalk_msg_relation(mid,owner_uid,other_uid,type,state,create_time) VALUES(?,?,?,?,?,?) `+healState,
		midStr, target, from, relTypeRev, relStateOK, createTime)
	return err
}

func (d *Dao) MarkMsgRevoked(mid int64) error {
	_, _, err := d.db.Exec(`UPDATE dtalk_msg_relation SET state = ? WHERE mid = ?`, 2, mid)
	if err != nil {
		d.log.Error("MarkMsgRevoked", "mid", mid, "err", err)
	}
	return err
}

func isDup(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "Duplicate") || strings.Contains(s, "1062")
}
