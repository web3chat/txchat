package record

import (
	"encoding/base64"
	"encoding/json"

	"github.com/web3chat/txchat/dtalk/gateway/api/v1/internal/model"
	xerror "github.com/web3chat/txchat/dtalk/pkg/error"
	"github.com/web3chat/txchat/dtalk/pkg/util"
	store "github.com/web3chat/txchat/dtalk/service/record/store/api"
	storeModel "github.com/web3chat/txchat/dtalk/service/record/store/model"
	"github.com/web3chat/txchat/imparse/proto"
)

func (l *Logic) GetPriRecord(req *model.GetPriRecordsReq) (*model.GetPriRecordsResp, error) {
	resp, err := l.svcCtx.StoreClient.GetRecordsAfterMid(l.ctx, &store.GetRecordsAfterMidReq{
		Tp:     proto.Channel_ToUser,
		Mid:    util.ToInt64(req.Mid),
		From:   req.FromId,
		Target: req.TargetId,
		Count:  req.RecordCount,
	})
	if err != nil {
		return nil, xerror.NewError(xerror.QueryFailed).SetExtMessage(err.Error())
	}

	res := &model.GetPriRecordsResp{
		RecordCount: len(resp.Records),
		Records:     toChatRecord(resp.Records),
	}

	return res, nil
}

// toChatRecord []MsgContent -> []Record , content json -> proto
func toChatRecord(msgs []*store.GetRecordReply) []*model.Record {
	Result := make([]*model.Record, len(msgs))
	for i, msg := range msgs {
		Result[i] = &model.Record{
			Mid:        msg.Mid,
			Seq:        msg.Seq,
			FromId:     msg.SenderId,
			TargetId:   msg.ReceiverId,
			MsgType:    int32(msg.MsgType),
			Content:    storeModel.ParseContent(msg.MsgType, []byte(msg.Content)),
			CreateTime: msg.CreateTime,
			Source:     parseSourceJSON(msg.Source),
		}
	}
	return Result
}

func parseSourceJSON(raw string) interface{} {
	if raw == "" {
		return nil
	}
	var v interface{}
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return nil
	}
	return v
}

func (l *Logic) GetSyncRecord(uid string, req *model.SyncRecordsReq) (*model.SyncRecordsResp, error) {
	resp, err := l.svcCtx.StoreClient.GetSyncRecordsAfterMid(l.ctx, &store.GetSyncRecordsAfterMidReq{
		Mid:   req.StartMid,
		Uid:   uid,
		Count: req.MaxCount,
	})
	if err != nil {
		return nil, xerror.NewError(xerror.QueryFailed).SetExtMessage(err.Error())
	}

	records := make([]string, len(resp.GetRecords()))
	for i, record := range resp.GetRecords() {
		records[i] = base64.StdEncoding.EncodeToString(record)
	}

	res := &model.SyncRecordsResp{
		RecordCount: len(resp.GetRecords()),
		Records:     records,
	}

	return res, nil
}
