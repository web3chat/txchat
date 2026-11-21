package service

import (
	"testing"
	"time"

	xtime "github.com/web3chat/txchat/dtalk/pkg/time"
	"github.com/web3chat/txchat/dtalk/service/shardkey/config"
)

func testConfig() *config.Config {
	cfg := config.Default()
	cfg.Code.Expire = xtime.Duration(5 * time.Minute)
	cfg.Code.RetryInterval = xtime.Duration(time.Minute)
	cfg.Code.MaxAttempts = 3
	cfg.Code.LockDuration = xtime.Duration(10 * time.Minute)
	return cfg
}

func TestSMSServicePhoneValidation(t *testing.T) {
	svc := NewSMSService(testConfig(), NewMemoryVerificationStore())

	validPhones := []string{"13800138000", "13900139000"}
	invalidPhones := []string{"123", "1380013800", "138001380000", "abc12345678"}

	for _, phone := range validPhones {
		if !isValidPhoneNumber(phone) {
			t.Errorf("phone %s should be valid", phone)
		}
	}
	for _, phone := range invalidPhones {
		if isValidPhoneNumber(phone) {
			t.Errorf("phone %s should be invalid", phone)
		}
	}

	code := svc.generateCode()
	if len(code) != testConfig().Code.Length {
		t.Errorf("code length = %d, want %d", len(code), testConfig().Code.Length)
	}
	for _, c := range code {
		if c < '0' || c > '9' {
			t.Errorf("code contains non-digit: %c", c)
		}
	}
}

func TestVerifyCodeWithPurpose(t *testing.T) {
	cfg := testConfig()
	store := NewMemoryVerificationStore()
	svc := NewSMSService(cfg, store)

	phone := "13800138000"
	rec := &VerificationCode{
		Code:      "123456",
		Phone:     phone,
		Purpose:   "backup",
		CreatedAt: time.Now(),
	}
	if err := store.Save(phone, rec, time.Duration(cfg.Code.Expire)); err != nil {
		t.Fatal(err)
	}

	valid, err := svc.VerifyCodeWithPurpose(phone, "123456", "get")
	if err == nil || valid {
		t.Fatal("purpose mismatch should fail")
	}

	valid, err = svc.VerifyCodeWithPurpose(phone, "123456", "backup")
	if err != nil || !valid {
		t.Fatalf("valid code should pass: valid=%v err=%v", valid, err)
	}

	// 一次性使用
	valid, err = svc.VerifyCodeWithPurpose(phone, "123456", "backup")
	if err == nil || valid {
		t.Fatal("code should not be reusable")
	}
}

func TestVerifyCodeAttemptsLock(t *testing.T) {
	cfg := testConfig()
	store := NewMemoryVerificationStore()
	svc := NewSMSService(cfg, store)

	phone := "13800138000"
	if err := store.Save(phone, &VerificationCode{
		Code: "123456", Phone: phone, Purpose: "backup", CreatedAt: time.Now(),
	}, time.Duration(cfg.Code.Expire)); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < cfg.Code.MaxAttempts; i++ {
		if _, err := svc.VerifyCodeWithPurpose(phone, "000000", "backup"); err == nil {
			t.Fatal("wrong code should fail")
		}
	}

	// 达到上限后验证码应被清除
	if rec, _ := store.Get(phone); rec != nil {
		t.Fatal("code should be cleared after too many attempts")
	}
}

func TestSendLock(t *testing.T) {
	store := NewMemoryVerificationStore()
	ok, err := store.TrySendLock("13800138000", time.Minute)
	if err != nil || !ok {
		t.Fatalf("first lock should succeed: ok=%v err=%v", ok, err)
	}
	ok, err = store.TrySendLock("13800138000", time.Minute)
	if err != nil || ok {
		t.Fatalf("second lock should be rejected: ok=%v err=%v", ok, err)
	}
}
