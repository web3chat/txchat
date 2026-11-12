package ossclient

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/web3chat/txchat/dtalk/service/square/internal/config"
	"github.com/web3chat/txchat/dtalk/service/square/pkg/auth"
)

var png1x1 = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
	0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4, 0x89, 0x00, 0x00, 0x00,
	0x0a, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00, 0x00, 0x00, 0x00, 0x49,
	0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
}

const testOSSPrivKey = "fa884fd1b47d9e9e8dc19e47dc1a794a524ce5d4ee1b82ec92b1ffc1f109c2b6"

func TestUploadURL(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"http://localhost:8888", "http://localhost:8888/oss/upload"},
		{"http://localhost:8888/", "http://localhost:8888/oss/upload"},
		{"http://localhost:8888/oss", "http://localhost:8888/oss/upload"},
		{"http://localhost:8888/oss/upload", "http://localhost:8888/oss/upload"},
	}
	for _, tc := range cases {
		got := config.OSSConfig{BaseURL: tc.in}.UploadURL()
		if got != tc.want {
			t.Fatalf("in=%s got=%s want=%s", tc.in, got, tc.want)
		}
	}
}

func TestImageExt(t *testing.T) {
	t.Parallel()
	ext, ok := imageExt("a.png", png1x1)
	if !ok || ext != "png" {
		t.Fatalf("got %s %v", ext, ok)
	}
	if _, ok := imageExt("a.txt", []byte("hello")); ok {
		t.Fatal("text should fail")
	}
}

func TestSignRoundTrip(t *testing.T) {
	t.Parallel()
	c := New(config.OSSConfig{
		BaseURL: "http://example/oss/upload",
		AppID:   "dtalk",
		PrivKey: testOSSPrivKey,
	})
	if !c.Enabled() {
		t.Fatal("signer should be enabled")
	}
	sig1, err := c.signOSS()
	if err != nil {
		t.Fatal(err)
	}
	uid1, err := auth.NewDefaultApiAuthenticator().Auth(sig1)
	if err != nil {
		t.Fatal(err)
	}
	if uid1 == "" {
		t.Fatal("empty uid")
	}
	sig2, err := c.signOSS()
	if err != nil {
		t.Fatal(err)
	}
	uid2, err := auth.NewDefaultApiAuthenticator().Auth(sig2)
	if err != nil {
		t.Fatal(err)
	}
	if uid1 != uid2 {
		t.Fatalf("signer should be stable: %s vs %s", uid1, uid2)
	}
	if c.SignerAddr() != uid1 {
		t.Fatalf("addr=%s uid=%s", c.SignerAddr(), uid1)
	}
}

func TestDeleteDisabledDoesNotRequest(t *testing.T) {
	t.Parallel()
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer srv.Close()
	c := New(config.OSSConfig{BaseURL: srv.URL, AppID: "dtalk", PrivKey: testOSSPrivKey, DeleteEnabled: false})
	if err := c.Delete(context.Background(), "https://cdn.example/a.png"); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("disabled delete must not hit OSS")
	}
}

func TestParseOSSDeleteAck(t *testing.T) {
	t.Parallel()
	if err := parseOSSDeleteAck([]byte(`{"result":0,"message":"ok"}`)); err != nil {
		t.Fatal(err)
	}
	if err := parseOSSDeleteAck(nil); err != nil {
		t.Fatal(err)
	}
	if err := parseOSSDeleteAck([]byte(`{"result":1,"message":"no"}`)); err == nil {
		t.Fatal("should fail")
	}
}

func TestInvalidPrivKeyDisablesClient(t *testing.T) {
	t.Parallel()
	c := New(config.OSSConfig{BaseURL: "http://example/oss/upload", AppID: "dtalk", PrivKey: "00"})
	if c.Enabled() {
		t.Fatal("short key should disable signer")
	}
}

func TestParseOSSAck(t *testing.T) {
	t.Parallel()
	raw, _ := json.Marshal(map[string]any{
		"result":  0,
		"message": "操作成功",
		"data":    map[string]string{"url": "https://cdn.example/a.png", "uri": "a.png"},
	})
	url, err := parseOSSAck(raw)
	if err != nil || url != "https://cdn.example/a.png" {
		t.Fatalf("got %s %v", url, err)
	}
}

func TestClientUpload(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("FZM-SIGNATURE") == "" {
			t.Error("missing FZM-SIGNATURE")
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Error(err)
		}
		if r.FormValue("appId") != "dtalk" {
			t.Errorf("appId=%s", r.FormValue("appId"))
		}
		if !strings.HasPrefix(r.FormValue("key"), "chatList/picture/") {
			t.Errorf("key=%s", r.FormValue("key"))
		}
		f, _, err := r.FormFile("file")
		if err != nil {
			t.Error(err)
		} else {
			io.Copy(io.Discard, f)
			f.Close()
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result":  0,
			"message": "操作成功",
			"data":    map[string]string{"url": "https://cdn.example/up.png"},
		})
	}))
	defer srv.Close()
	c := New(config.OSSConfig{BaseURL: srv.URL + "/oss/upload", AppID: "dtalk", PrivKey: testOSSPrivKey})
	url, err := c.Upload(context.Background(), "picture", "a.png", int64(len(png1x1)), bytes.NewReader(png1x1))
	if err != nil {
		t.Fatal(err)
	}
	if url != "https://cdn.example/up.png" {
		t.Fatalf("url=%s", url)
	}
}
