package record

import answer "github.com/web3chat/txchat/dtalk/service/record/answer/api"

func (l *Logic) Push(key, from string, body []byte) (int64, uint64, error) {
	return l.svcCtx.AnswerClient.PushCommonMsg(l.ctx, key, from, body, answer.MessageOrigin_MESSAGE_ORIGIN_UNSPECIFIED)
}

// PushWithOrigin 带消息来源标记；push2 使用 MESSAGE_ORIGIN_PUSH2。
func (l *Logic) PushWithOrigin(key, from string, body []byte, origin answer.MessageOrigin) (int64, uint64, error) {
	return l.svcCtx.AnswerClient.PushCommonMsg(l.ctx, key, from, body, origin)
}
