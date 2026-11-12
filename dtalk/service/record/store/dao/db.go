package dao

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/web3chat/txchat/dtalk/pkg/mysql"
	"github.com/web3chat/txchat/dtalk/pkg/util"
	"github.com/web3chat/txchat/dtalk/service/record/store/model"
)

const (
	_InsertMsgContent  = `INSERT INTO dtalk_msg_content(mid,seq,sender_id,receiver_id,msg_type,content,create_time,source,reference) VALUES(?,?,?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE mid=mid`
	_InsertMsgRelation = `INSERT INTO dtalk_msg_relation(mid,owner_uid,other_uid,type,state,create_time) VALUES(?,?,?,?,?,?) ON DUPLICATE KEY UPDATE mid=mid`
	_SetRecordState    = `UPDATE dtalk_msg_relation SET state = ? WHERE owner_uid = ? AND mid = ?`
	_DelMsgContent     = `DELETE FROM dtalk_msg_content WHERE mid = ?`

	_InsertGroupMsgContent        = `INSERT INTO dtalk_group_msg_content(mid,seq,sender_id,receiver_id,msg_type,content,create_time,source,reference) VALUES(?,?,?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE mid=mid`
	_InsertGroupMsgRelationPrefix = "INSERT INTO dtalk_group_msg_relation(mid,owner_uid,other_uid,type,state,create_time) VALUES%s ON DUPLICATE KEY UPDATE mid=mid"
	_SetGroupRecordState          = `UPDATE dtalk_group_msg_relation SET state = ? WHERE owner_uid = ? AND mid = ?`
	_DelGroupMsgContent           = `DELETE FROM dtalk_group_msg_content WHERE mid = ?`

	//_GetUnReceiveMsg      = `SELECT co.mid as mid,co.seq as seq,co.sender_id as sender_id,co.receiver_id as receiver_id,co.msg_type as msg_type,co.content as content,co.create_time as create_time,co.source as source,co.reference as reference FROM dtalk_msg_relation AS re RIGHT JOIN dtalk_msg_content AS co ON re.mid=co.mid WHERE re.owner_uid=? AND re.type=? AND re.state=? ORDER BY re.mid`
	_GetUnReceiveMsg = `SELECT co.mid as mid,co.seq as seq,co.sender_id as sender_id,co.receiver_id as receiver_id,co.msg_type as msg_type,co.content as content,co.create_time as create_time,co.source as source,co.reference as reference FROM dtalk_msg_relation AS re RIGHT JOIN dtalk_msg_content AS co ON re.mid=co.mid WHERE re.owner_uid=? AND re.state=? ORDER BY re.mid`
	//_GetUnReceiveGroupMsg = `SELECT co.mid as mid,co.seq as seq,co.sender_id as sender_id,co.receiver_id as receiver_id,co.msg_type as msg_type,co.content as content,co.create_time as create_time,co.source as source,co.reference as reference FROM dtalk_group_msg_relation AS re RIGHT JOIN dtalk_group_msg_content AS co ON re.mid=co.mid WHERE re.owner_uid=? AND re.type=? AND re.state=? ORDER BY re.mid`
	_GetUnReceiveGroupMsg = `SELECT co.mid as mid,co.seq as seq,co.sender_id as sender_id,co.receiver_id as receiver_id,co.msg_type as msg_type,co.content as content,co.create_time as create_time,co.source as source,co.reference as reference FROM dtalk_group_msg_relation AS re RIGHT JOIN dtalk_group_msg_content AS co ON re.mid=co.mid WHERE re.owner_uid=? AND re.state=? ORDER BY re.mid`
	_GetUserMsg           = `SELECT co.mid as mid,co.seq as seq,co.sender_id as sender_id,co.receiver_id as receiver_id,co.msg_type as msg_type,co.content as content,co.create_time as create_time,co.source as source,co.reference as reference FROM dtalk_msg_relation AS re RIGHT JOIN dtalk_msg_content AS co ON re.mid=co.mid WHERE re.owner_uid=? ORDER BY re.mid LIMIT ?,?`
	_GetUserMsgAfter      = `SELECT co.mid as mid,co.seq as seq,co.sender_id as sender_id,co.receiver_id as receiver_id,co.msg_type as msg_type,co.content as content,co.create_time as create_time,co.source as source,co.reference as reference FROM dtalk_msg_relation AS re RIGHT JOIN dtalk_msg_content AS co ON re.mid=co.mid WHERE re.owner_uid=? AND co.mid>? ORDER BY re.mid DESC LIMIT ?,?`
	_GetGroupMsgAfter     = `SELECT co.mid as mid,co.seq as seq,co.sender_id as sender_id,co.receiver_id as receiver_id,co.msg_type as msg_type,co.content as content,co.create_time as create_time,co.source as source,co.reference as reference FROM dtalk_group_msg_relation AS re RIGHT JOIN dtalk_group_msg_content AS co ON re.mid=co.mid WHERE re.owner_uid=? AND co.mid>? ORDER BY re.mid DESC LIMIT ?,?`

	_GetSpecifyRecords      = `SELECT co.mid as mid,co.seq as seq,co.sender_id as sender_id,co.receiver_id as receiver_id,co.msg_type as msg_type,co.content as content,co.create_time as create_time,co.source as source,co.reference as reference FROM dtalk_msg_relation AS re RIGHT JOIN dtalk_msg_content AS co ON re.mid=co.mid WHERE re.mid = ?`
	_GetSpecifyGroupRecords = `SELECT co.mid as mid,co.seq as seq,co.sender_id as sender_id,co.receiver_id as receiver_id,co.msg_type as msg_type,co.content as content,co.create_time as create_time,co.source as source,co.reference as reference FROM dtalk_group_msg_relation AS re RIGHT JOIN dtalk_group_msg_content AS co ON re.mid=co.mid WHERE re.mid = ?`

	_GetMsgBySeq = `SELECT mid,seq,sender_id,receiver_id,msg_type,content,create_time,source,reference FROM dtalk_msg_content WHERE sender_id=? AND seq=?`

	_IncMsgVersion = `INSERT INTO dtalk_msg_version(uid,version) VALUES(?,?) ON DUPLICATE KEY UPDATE version=version+1`
	_GetMsgVersion = `SELECT version FROM dtalk_msg_version WHERE uid = ?`

	_GetPriRecords = `SELECT co.mid as mid,co.seq as seq,co.sender_id as sender_id,co.receiver_id as receiver_id,co.msg_type as msg_type,co.content as content,co.create_time as create_time,co.source as source,co.reference as reference FROM dtalk_msg_relation AS re RIGHT JOIN dtalk_msg_content AS co ON re.mid=co.mid WHERE re.owner_uid=? AND re.other_uid=? AND re.mid<? AND re.state=1 ORDER BY re.mid desc LIMIT 0,?`

	_GetRelationsByMid      = `SELECT mid,owner_uid,other_uid,type,state,create_time FROM dtalk_msg_relation WHERE mid=?`
	_DeletePrivateContent   = `DELETE FROM dtalk_msg_content WHERE mid=?`
	_DeletePrivateRelations = `DELETE FROM dtalk_msg_relation WHERE mid=?`

	_GetGroupMsgContent                 = `SELECT mid,seq,sender_id,receiver_id,msg_type,content,create_time,source,reference FROM dtalk_group_msg_content WHERE mid=?`
	_GetGroupMsgContentForUpdate        = `SELECT mid,seq,sender_id,receiver_id,msg_type,content,create_time,source,reference FROM dtalk_group_msg_content WHERE mid=? FOR UPDATE`
	_GetGroupReceiverRelationsForUpdate = `SELECT mid,owner_uid,other_uid,type,state,create_time FROM dtalk_group_msg_relation WHERE mid=? AND type=? FOR UPDATE`
	_GetGroupOwnerRelationForUpdate     = `SELECT mid,owner_uid,other_uid,type,state,create_time FROM dtalk_group_msg_relation WHERE mid=? AND owner_uid=? FOR UPDATE`
	_DeleteGroupReceiverRelation        = `DELETE FROM dtalk_group_msg_relation WHERE mid=? AND owner_uid=? AND type=?`
	_DeleteGroupRelationsByMid          = `DELETE FROM dtalk_group_msg_relation WHERE mid=?`
	_GetGroupRelationsByMid             = `SELECT mid,owner_uid,other_uid,type,state,create_time FROM dtalk_group_msg_relation WHERE mid=?`
)

func convertMsgContent(m map[string]string) *model.MsgContent {
	return &model.MsgContent{
		Mid:        m["mid"],
		Seq:        m["seq"],
		SenderId:   m["sender_id"],
		ReceiverId: m["receiver_id"],
		MsgType:    util.ToUInt32(m["msg_type"]),
		Content:    m["content"],
		CreateTime: uint64(util.ToInt64(m["create_time"])),
		Source:     m["source"],
		Reference:  m["reference"],
	}
}

func (d *Dao) AppendMsgContent(tx *mysql.MysqlTx, m *model.MsgContent) (int64, int64, error) {
	num, lastId, err := tx.Exec(_InsertMsgContent,
		m.Mid, m.Seq, m.SenderId, m.ReceiverId, m.MsgType, m.Content, m.CreateTime, m.Source, m.Reference)
	return num, lastId, err
}

func (d *Dao) AppendMsgRelation(tx *mysql.MysqlTx, m *model.MsgRelation) (int64, int64, error) {
	num, lastId, err := tx.Exec(_InsertMsgRelation,
		m.Mid, m.OwnerUid, m.OtherUid, m.Type, m.State, m.CreateTime)
	return num, lastId, err
}

func (d *Dao) MarkMsgReceived(uid string, mid int64) (int64, int64, error) {
	num, lastId, err := d.conn.Exec(_SetRecordState, model.Received, uid, mid)
	return num, lastId, err
}

func (d *Dao) DelMsgContent(mid int64) (int64, int64, error) {
	num, lastId, err := d.conn.Exec(_DelMsgContent, mid)
	return num, lastId, err
}

func (d *Dao) UnReceiveMsg(uid string) ([]*model.MsgContent, error) {
	maps, err := d.conn.Query(_GetUnReceiveMsg, uid, model.UnReceive)
	if err != nil {
		return nil, err
	}
	ret := make([]*model.MsgContent, len(maps))
	for i, m := range maps {
		ret[i] = convertMsgContent(m)
	}
	return ret, err
}

func (d *Dao) UserLastMsg(uid string, num int64) ([]*model.MsgContent, error) {
	maps, err := d.conn.Query(_GetUserMsg, uid, 0, num)
	if err != nil {
		return nil, err
	}
	ret := make([]*model.MsgContent, len(maps))
	for i, m := range maps {
		ret[i] = convertMsgContent(m)
	}
	return ret, err
}

func (d *Dao) UserMsgAfter(uid string, startMid, count int64) ([]*model.MsgContent, error) {
	maps, err := d.conn.Query(_GetUserMsgAfter, uid, startMid, 0, count)
	if err != nil {
		return nil, err
	}
	ret := make([]*model.MsgContent, len(maps))
	for i, m := range maps {
		ret[i] = convertMsgContent(m)
	}
	return ret, err
}

func (d *Dao) GetMsgBySeq(senderUid, seq string) (*model.MsgContent, error) {
	maps, err := d.conn.Query(_GetMsgBySeq, senderUid, seq)
	if err != nil {
		return nil, err
	}
	if len(maps) < 1 {
		return nil, nil
	}
	m := maps[0]
	return convertMsgContent(m), err
}

func (d *Dao) IncMsgVersion(uid string) (uint64, error) {
	tx, err := d.conn.NewTx()
	if err != nil {
		return 0, err
	}
	_, _, err = tx.Exec(_IncMsgVersion, uid, 0)
	if err != nil {
		tx.RollBack()
		return 0, err
	}
	m, err := tx.Query(_GetMsgVersion, uid)
	if err != nil {
		tx.RollBack()
		return 0, err
	}
	if len(m) < 1 {
		tx.RollBack()
		return 0, errors.New("record not find")
	}
	err = tx.Commit()
	if err != nil {
		return 0, err
	}
	ver, err := strconv.ParseUint(m[0]["version"], 10, 64)
	if err != nil {
		return 0, err
	}
	return ver, err
}

//group
func (d *Dao) AppendGroupMsgContent(tx *mysql.MysqlTx, m *model.MsgContent) (int64, int64, error) {
	num, lastId, err := tx.Exec(_InsertGroupMsgContent,
		m.Mid, m.Seq, m.SenderId, m.ReceiverId, m.MsgType, m.Content, m.CreateTime, m.Source, m.Reference)
	return num, lastId, err
}

func (d *Dao) AppendGroupMsgRelation(tx *mysql.MysqlTx, m []*model.MsgRelation) (int64, int64, error) {
	//element should not empty
	if len(m) == 0 {
		return 0, 0, nil
	}
	var values []interface{}
	condition := ""
	for _, row := range m {
		condition += "(?,?,?,?,?,?),"
		values = append(values, row.Mid, row.OwnerUid, row.OtherUid, row.Type, row.State, row.CreateTime)
	}
	//trim the last ,
	condition = strings.TrimSuffix(condition, ",")
	//prepare the statement and exec
	num, lastId, err := tx.PrepareExec(fmt.Sprintf(_InsertGroupMsgRelationPrefix, condition), values...)
	return num, lastId, err
}

func (d *Dao) MarkGroupMsgReceived(uid string, mid int64) (int64, int64, error) {
	num, lastId, err := d.conn.Exec(_SetGroupRecordState, model.Received, uid, mid)
	return num, lastId, err
}

func (d *Dao) DelGroupMsgContent(mid int64) (int64, int64, error) {
	num, lastId, err := d.conn.Exec(_DelGroupMsgContent, mid)
	return num, lastId, err
}

func (d *Dao) UnReceiveGroupMsg(uid string) ([]*model.MsgContent, error) {
	maps, err := d.conn.Query(_GetUnReceiveGroupMsg, uid, model.UnReceive)
	if err != nil {
		return nil, err
	}
	ret := make([]*model.MsgContent, len(maps))
	for i, m := range maps {
		ret[i] = convertMsgContent(m)
	}
	return ret, err
}

func (d *Dao) GroupMsgAfter(uid string, startMid, count int64) ([]*model.MsgContent, error) {
	maps, err := d.conn.Query(_GetGroupMsgAfter, uid, startMid, 0, count)
	if err != nil {
		return nil, err
	}
	ret := make([]*model.MsgContent, len(maps))
	for i, m := range maps {
		ret[i] = convertMsgContent(m)
	}
	return ret, err
}

// get record

// GetPriRecord 获得聊天记录
func (d *Dao) GetPriRecord(fromId, targetId string, mid int64, recordCount int64) ([]*model.MsgContent, error) {
	maps, err := d.conn.Query(_GetPriRecords, fromId, targetId, mid, recordCount)
	if err != nil {
		return nil, err
	}
	res := make([]*model.MsgContent, len(maps))
	for i, m := range maps {
		res[i] = convertMsgContent(m)
	}

	return res, nil
}

func convertMsgRelation(m map[string]string) *model.MsgRelation {
	return &model.MsgRelation{
		Mid:        m["mid"],
		OwnerUid:   m["owner_uid"],
		OtherUid:   m["other_uid"],
		Type:       uint8(util.ToUInt32(m["type"])),
		State:      uint8(util.ToUInt32(m["state"])),
		CreateTime: uint64(util.ToInt64(m["create_time"])),
	}
}

// GetRelationsByMid 查询某 mid 在私聊关系表中的全部行。
func (d *Dao) GetRelationsByMid(mid int64) ([]*model.MsgRelation, error) {
	maps, err := d.conn.Query(_GetRelationsByMid, mid)
	if err != nil {
		return nil, err
	}
	ret := make([]*model.MsgRelation, len(maps))
	for i, m := range maps {
		ret[i] = convertMsgRelation(m)
	}
	return ret, nil
}

// DeletePrivateMessageFully 事务删除私聊 content 与该 mid 全部 relation。
func (d *Dao) DeletePrivateMessageFully(mid int64) error {
	tx, err := d.NewTx()
	if err != nil {
		return err
	}
	if _, _, err = tx.Exec(_DeletePrivateContent, mid); err != nil {
		tx.RollBack()
		return err
	}
	if _, _, err = tx.Exec(_DeletePrivateRelations, mid); err != nil {
		tx.RollBack()
		return err
	}
	if err = tx.Commit(); err != nil {
		tx.RollBack()
		return err
	}
	return nil
}

// GetRecord 获得指定聊天记录
func (d *Dao) GetSpecifyRecord(mid int64) (*model.MsgContent, error) {
	maps, err := d.conn.Query(_GetSpecifyRecords, mid)
	if err != nil {
		return nil, err
	}
	if len(maps) < 1 {
		return nil, model.ErrRecordNotFind
	}
	return convertMsgContent(maps[0]), nil
}

func (d *Dao) GetSpecifyGroupRecord(mid int64) (*model.MsgContent, error) {
	maps, err := d.conn.Query(_GetSpecifyGroupRecords, mid)
	if err != nil {
		return nil, err
	}
	if len(maps) < 1 {
		return nil, model.ErrRecordNotFind
	}
	return convertMsgContent(maps[0]), nil
}

func convertTxRelations(maps []map[string]string) []*model.MsgRelation {
	ret := make([]*model.MsgRelation, len(maps))
	for i, m := range maps {
		ret[i] = convertMsgRelation(m)
	}
	return ret
}

// GetGroupMsgContentForUpdate 事务内锁群正文，不 JOIN relation，以便最后一人留下 content 孤行后仍能读到。
func (d *Dao) GetGroupMsgContentForUpdate(tx *mysql.MysqlTx, mid int64) (*model.MsgContent, error) {
	maps, err := tx.Query(_GetGroupMsgContentForUpdate, mid)
	if err != nil {
		return nil, err
	}
	if len(maps) < 1 {
		return nil, model.ErrRecordNotFind
	}
	return convertMsgContent(maps[0]), nil
}

// GetGroupReceiverRelationsForUpdate 加锁读取该 mid 全部收方关系行。
func (d *Dao) GetGroupReceiverRelationsForUpdate(tx *mysql.MysqlTx, mid int64) ([]*model.MsgRelation, error) {
	maps, err := tx.Query(_GetGroupReceiverRelationsForUpdate, mid, model.Rev)
	if err != nil {
		return nil, err
	}
	return convertTxRelations(maps), nil
}

// GetGroupOwnerRelationForUpdate 加锁读取操作者在该 mid 上的关系行；没有行时返回 nil, nil。
func (d *Dao) GetGroupOwnerRelationForUpdate(tx *mysql.MysqlTx, mid int64, ownerUID string) (*model.MsgRelation, error) {
	maps, err := tx.Query(_GetGroupOwnerRelationForUpdate, mid, ownerUID)
	if err != nil {
		return nil, err
	}
	if len(maps) < 1 {
		return nil, nil
	}
	return convertMsgRelation(maps[0]), nil
}

func (d *Dao) DeleteGroupReceiverRelation(tx *mysql.MysqlTx, mid int64, ownerUID string) error {
	_, _, err := tx.Exec(_DeleteGroupReceiverRelation, mid, ownerUID, model.Rev)
	return err
}

func (d *Dao) DeleteGroupRelationsByMid(tx *mysql.MysqlTx, mid int64) error {
	_, _, err := tx.Exec(_DeleteGroupRelationsByMid, mid)
	return err
}

// GetGroupRelationsByMid 查询某 mid 在群关系表中的全部行（联调用）。
func (d *Dao) GetGroupRelationsByMid(mid int64) ([]*model.MsgRelation, error) {
	maps, err := d.conn.Query(_GetGroupRelationsByMid, mid)
	if err != nil {
		return nil, err
	}
	return convertTxRelations(maps), nil
}

// GetGroupMsgContent 直接读群正文，不 JOIN relation。
func (d *Dao) GetGroupMsgContent(mid int64) (*model.MsgContent, error) {
	maps, err := d.conn.Query(_GetGroupMsgContent, mid)
	if err != nil {
		return nil, err
	}
	if len(maps) < 1 {
		return nil, model.ErrRecordNotFind
	}
	return convertMsgContent(maps[0]), nil
}
