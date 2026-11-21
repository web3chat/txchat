package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	t.Setenv("MYSQL_PASSWORD", "mysql-pass")
	t.Setenv("SUBMAIL_APP_ID", "appid")
	t.Setenv("SUBMAIL_APP_KEY", "appkey")
	t.Setenv("SUBMAIL_SIGNATURE", "sign")
	t.Setenv("SHARDKEY_ENCRYPT_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("SHARDKEY_IOS_APP_TOKEN", "ios-token")
	t.Setenv("SHARDKEY_ANDROID_APP_TOKEN", "android-token")

	content := `
Env = "release"
[HttpServer]
Addr = "0.0.0.0:18006"
[MySQL]
Host = "mysql"
Port = 3306
User = "root"
Pwd = "${MYSQL_PASSWORD}"
Db = "dtalk"
[SMS]
AppID = "${SUBMAIL_APP_ID}"
AppKey = "${SUBMAIL_APP_KEY}"
Signature = "${SUBMAIL_SIGNATURE}"
[Code]
Length = 6
Expire = "5m"
RetryInterval = "60s"
[Security]
EncryptKey = "${SHARDKEY_ENCRYPT_KEY}"
[APIKeys]
ios = "${SHARDKEY_IOS_APP_TOKEN}"
android = "${SHARDKEY_ANDROID_APP_TOKEN}"
`
	path := filepath.Join(t.TempDir(), "shardkey.toml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MySQL.Pwd != "mysql-pass" {
		t.Errorf("mysql password not expanded: %q", cfg.MySQL.Pwd)
	}
	if cfg.SMS.AppID != "appid" || cfg.SMS.AppKey != "appkey" {
		t.Errorf("sms credentials not expanded: %+v", cfg.SMS)
	}
	if time.Duration(cfg.Code.Expire) != 5*time.Minute {
		t.Errorf("code expire = %v", time.Duration(cfg.Code.Expire))
	}
	if cfg.APIKeys["ios"] != "ios-token" {
		t.Errorf("api keys not expanded: %+v", cfg.APIKeys)
	}
}

func TestLoadConfigMissingEncryptKey(t *testing.T) {
	t.Setenv("MYSQL_PASSWORD", "mysql-pass")
	t.Setenv("SUBMAIL_APP_ID", "appid")
	t.Setenv("SUBMAIL_APP_KEY", "appkey")
	t.Setenv("SHARDKEY_ENCRYPT_KEY", "")

	content := `
Env = "release"
[HttpServer]
Addr = "0.0.0.0:18006"
[MySQL]
Host = "mysql"
Pwd = "${MYSQL_PASSWORD}"
Db = "dtalk"
[SMS]
AppID = "${SUBMAIL_APP_ID}"
AppKey = "${SUBMAIL_APP_KEY}"
[Security]
EncryptKey = "${SHARDKEY_ENCRYPT_KEY}"
`
	path := filepath.Join(t.TempDir(), "shardkey.toml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected error for missing encrypt key")
	}
}
