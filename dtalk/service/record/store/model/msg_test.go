package model

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/golang/protobuf/proto"
	xproto "github.com/web3chat/txchat/imparse/proto"
)

func TestJsonUnmarshal_TextMsg(t *testing.T) {
	t.Parallel()

	const want = "你好"
	textMsg := &xproto.TextMsg{Content: want}
	protoBytes, err := proto.Marshal(textMsg)
	if err != nil {
		t.Fatalf("marshal TextMsg: %v", err)
	}
	jsonBytes, err := json.Marshal(textMsg)
	if err != nil {
		t.Fatalf("marshal TextMsg json: %v", err)
	}
	base64Bytes := []byte(base64.StdEncoding.EncodeToString(protoBytes))

	tests := []struct {
		name string
		data []byte
	}{
		{name: "json", data: jsonBytes},
		{name: "protobuf", data: protoBytes},
		{name: "base64_protobuf", data: base64Bytes},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := JsonUnmarshal(uint32(xproto.MsgType_Text), tt.data)
			if got == nil {
				t.Fatal("JsonUnmarshal returned nil")
			}
			text, ok := got.(*xproto.TextMsg)
			if !ok {
				t.Fatalf("unexpected type %T", got)
			}
			if text.Content != want {
				t.Fatalf("Content=%q, want %q", text.Content, want)
			}
		})
	}
}

func TestJsonUnmarshal_ParseCommonMsgRoundTrip(t *testing.T) {
	t.Parallel()

	const want = "round-trip"
	textMsg := &xproto.TextMsg{Content: want}
	protoBytes, err := proto.Marshal(textMsg)
	if err != nil {
		t.Fatalf("marshal TextMsg: %v", err)
	}

	stored := ParseCommonMsg(&xproto.Common{
		MsgType: xproto.MsgType_Text,
		Msg:     protoBytes,
	})
	got := JsonUnmarshal(uint32(xproto.MsgType_Text), stored)
	if got == nil {
		t.Fatal("JsonUnmarshal returned nil")
	}
	text, ok := got.(*xproto.TextMsg)
	if !ok {
		t.Fatalf("unexpected type %T", got)
	}
	if text.Content != want {
		t.Fatalf("Content=%q, want %q", text.Content, want)
	}
}

func TestParseContent_JSONFallback(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"content":"你好","mention":[]}`)
	got := ParseContent(uint32(xproto.MsgType_Text), raw)
	if got == nil {
		t.Fatal("ParseContent returned nil")
	}
	m, ok := got.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected type %T", got)
	}
	if m["content"] != "你好" {
		t.Fatalf("content=%v", m["content"])
	}
}

func TestParseContent_JSONStringBase64(t *testing.T) {
	t.Parallel()

	const want = "hello-base64"
	textMsg := &xproto.TextMsg{Content: want}
	protoBytes, err := proto.Marshal(textMsg)
	if err != nil {
		t.Fatalf("marshal TextMsg: %v", err)
	}
	encoded, err := json.Marshal(base64.StdEncoding.EncodeToString(protoBytes))
	if err != nil {
		t.Fatalf("marshal base64 string: %v", err)
	}

	got := ParseContent(uint32(xproto.MsgType_Text), encoded)
	if got == nil {
		t.Fatal("ParseContent returned nil")
	}
	m, ok := got.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected type %T", got)
	}
	if m["content"] != want {
		t.Fatalf("content=%v, want %q", m["content"], want)
	}
}

func TestParseContent_OpaqueBase64Payload(t *testing.T) {
	t.Parallel()

	// 模拟 parse() 在 proto 解析失败时存入的 base64(加密二进制)，无法还原为 TextMsg。
	encrypted := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	stored := []byte(base64.StdEncoding.EncodeToString(encrypted))

	got := ParseContent(uint32(xproto.MsgType_Text), stored)
	if got == nil {
		t.Fatal("ParseContent returned nil")
	}
	m, ok := got.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected type %T", got)
	}
	content, ok := m["content"].(string)
	if !ok || content == "" {
		t.Fatalf("content=%v", m["content"])
	}
}

func TestParseContent_EmptyContent(t *testing.T) {
	t.Parallel()

	if got := ParseContent(uint32(xproto.MsgType_Text), nil); got != nil {
		t.Fatalf("ParseContent=%v, want nil", got)
	}
}

func TestJsonUnmarshal_InvalidContent(t *testing.T) {
	t.Parallel()

	got := JsonUnmarshal(uint32(xproto.MsgType_Text), []byte("not-a-valid-message"))
	if got != nil {
		t.Fatalf("JsonUnmarshal=%v, want nil", got)
	}
}

func TestParseSource(t *testing.T) {
	type args struct {
		m *xproto.Common
	}
	tests := []struct {
		name string
		args args
		want []byte
	}{
		{
			name: "",
			args: args{
				m: &xproto.Common{
					ChannelType: 0,
					Mid:         0,
					Seq:         "",
					From:        "",
					Target:      "",
					MsgType:     0,
					Msg:         nil,
					Datetime:    0,
					Source:      nil,
				},
			},
			want: nil,
		}, {
			name: "",
			args: args{
				m: &xproto.Common{
					ChannelType: 0,
					Mid:         0,
					Seq:         "",
					From:        "",
					Target:      "",
					MsgType:     0,
					Msg:         nil,
					Datetime:    0,
					Source:      &xproto.Source{},
				},
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Log(string(ParseSource(tt.args.m)))
		})
	}
}
