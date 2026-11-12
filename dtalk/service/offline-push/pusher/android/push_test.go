package android

import (
	"github.com/web3chat/txchat/dtalk/service/offline-push/pusher"
	"testing"
)

func Test_androidPusher_SinglePush(t1 *testing.T) {
	type fields struct {
		AppKey          string
		AppMasterSecret string
		MiActivity      string
		environment     string
	}
	type args struct {
		deviceToken string
		title       string
		text        string
		extra       *pusher.Extra
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "test android offline push",
			fields: fields{
				AppKey:          "your-umeng-app-key",
				AppMasterSecret: "your-umeng-master-secret",
				MiActivity:      "",
				environment:     "debug",
			},
			args: args{
				deviceToken: "your-device-token",
				title:       "测试title",
				text:        "测试text",
				extra: &pusher.Extra{
					Address:     "1FdnxKR4r952x2HQA2BTTpFH6tgHYYNs3M",
					ChannelType: 0,
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			t := &androidPusher{
				AppKey:          tt.fields.AppKey,
				AppMasterSecret: tt.fields.AppMasterSecret,
				MiActivity:      tt.fields.MiActivity,
				environment:     tt.fields.environment,
			}
			if err := t.SinglePush(tt.args.deviceToken, tt.args.title, tt.args.text, tt.args.extra); (err != nil) != tt.wantErr {
				t1.Errorf("SinglePush() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
