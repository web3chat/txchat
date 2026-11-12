package email

import (
	"github.com/web3chat/txchat/dtalk/service/backup/model"
	"testing"
)

/*
[smsConfig]
url="http://<host:port>/send/sms2"
codeType="chat_notice"
mobile=[""]
*/
func Test_Send(t *testing.T) {
	url := "http://localhost"
	appkey := "chat33pro"
	secretKey := "eQXXMphNFHQL4YeW"
	msg := "FzmRandom5"

	email := NewEmail(url, appkey, secretKey, msg)
	params := map[string]string{
		model.ParamEmail:    "test@example.com",
		model.ParamCodeType: "bind_policebook",
	}
	rlt, err := email.Send(params)
	if err != nil {
		t.Error(err)
		return
	}
	t.Log("success", "rlt", rlt)
}

func Test_ValidateCode(t *testing.T) {
	url := "http://localhost"
	appkey := "chat33pro"
	secretKey := "eQXXMphNFHQL4YeW"
	msg := "FzmRandom5"

	email := NewEmail(url, appkey, secretKey, msg)
	params := map[string]string{
		model.ParamEmail:    "test@example.com",
		model.ParamCode:     "17091",
		model.ParamCodeType: "bind_policebook",
	}
	err := email.ValidateCode(params)
	if err != nil {
		t.Error(err)
		return
	}
	t.Log("success")
}
