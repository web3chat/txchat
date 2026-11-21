package service

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/web3chat/txchat/dtalk/service/shardkey/config"
)

// SMSService 短信服务：负责发送验证码并校验（验证码存 Redis / 内存实现）。
type SMSService struct {
	cfg   *config.Config
	store VerificationStore
}

func NewSMSService(cfg *config.Config, store VerificationStore) *SMSService {
	return &SMSService{cfg: cfg, store: store}
}

var validPurposes = map[string]string{
	"backup": "备份密钥分片",
	"reset":  "重置密钥分片",
	"get":    "获取密钥分片",
	"delete": "注销密钥分片",
}

// PurposeName 返回用途的中文名，供 handler 校验/提示使用。
func PurposeName(purpose string) (string, bool) {
	name, ok := validPurposes[purpose]
	return name, ok
}

// SendVerificationCode 发送验证码。
func (s *SMSService) SendVerificationCode(phone, purpose string) error {
	if !isValidPhoneNumber(phone) {
		return fmt.Errorf("invalid phone number format")
	}
	purposeName, ok := validPurposes[purpose]
	if !ok {
		return fmt.Errorf("invalid purpose, must be one of: backup, reset, get, delete")
	}

	// 同手机号发送间隔，Redis SET NX 保证多实例下也只有一次成功
	locked, err := s.store.TrySendLock(phone, time.Duration(s.cfg.Code.RetryInterval))
	if err != nil {
		return fmt.Errorf("check send interval failed: %w", err)
	}
	if !locked {
		return fmt.Errorf("please wait before requesting another code")
	}

	code := s.generateCode()
	if err := s.sendSMSWithPurpose(phone, code, purposeName); err != nil {
		// 发送失败释放锁，允许用户重试
		_ = s.store.ReleaseSendLock(phone)
		logrus.WithFields(logrus.Fields{
			"phone":   maskPhone(phone),
			"purpose": purpose,
		}).WithError(err).Error("send verification sms failed")
		return fmt.Errorf("failed to send SMS")
	}

	rec := &VerificationCode{
		Code:      code,
		Phone:     phone,
		Purpose:   purpose,
		CreatedAt: time.Now(),
	}
	if err := s.store.Save(phone, rec, time.Duration(s.cfg.Code.Expire)); err != nil {
		return fmt.Errorf("save verification code failed: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"phone":   maskPhone(phone),
		"purpose": purpose,
	}).Info("verification code sent")
	return nil
}

// VerifyCode 不校验用途，向后兼容。
func (s *SMSService) VerifyCode(phone, code string) (bool, error) {
	return s.VerifyCodeWithPurpose(phone, code, "")
}

// VerifyCodeWithPurpose 校验验证码和用途；成功后验证码立即失效。
func (s *SMSService) VerifyCodeWithPurpose(phone, code, purpose string) (bool, error) {
	rec, err := s.store.Get(phone)
	if err != nil {
		return false, fmt.Errorf("read verification code failed: %w", err)
	}
	if rec == nil {
		return false, fmt.Errorf("verification code not found")
	}
	if time.Since(rec.CreatedAt) > time.Duration(s.cfg.Code.Expire) {
		_ = s.store.Delete(phone)
		return false, fmt.Errorf("verification code expired")
	}
	if purpose != "" && rec.Purpose != purpose {
		return false, fmt.Errorf("verification code purpose mismatch")
	}
	if rec.Code != code {
		n, ierr := s.store.IncrAttempts(phone, time.Duration(s.cfg.Code.LockDuration))
		if ierr == nil && n >= int64(s.cfg.Code.MaxAttempts) {
			_ = s.store.Delete(phone)
			_ = s.store.DeleteAttempts(phone)
			return false, fmt.Errorf("too many invalid attempts, please request a new code")
		}
		return false, fmt.Errorf("invalid verification code")
	}

	// 一次性使用
	_ = s.store.Delete(phone)
	_ = s.store.DeleteAttempts(phone)
	return true, nil
}

// sendSMSWithPurpose 调用赛邮 API 发送验证码短信。
func (s *SMSService) sendSMSWithPurpose(phone, code, purposeName string) error {
	content := fmt.Sprintf(s.cfg.SMS.Template, code, purposeName)
	return s.sendSMS(phone, content)
}

// SendCustomSMS 发送自定义内容短信（用于把分片密钥发到本机）。
func (s *SMSService) SendCustomSMS(phone, content string) error {
	if !isValidPhoneNumber(phone) {
		return fmt.Errorf("invalid phone number format")
	}
	return s.sendSMS(phone, content)
}

func (s *SMSService) sendSMS(phone, content string) error {
	// debug 环境不真正发短信，避免本地/联调产生费用；release 必须配置真实凭证。
	if s.cfg.Env != "release" {
		logrus.WithFields(logrus.Fields{
			"phone":   maskPhone(phone),
			"content": content,
		}).Info("SMS mock (non-release env), not sent")
		return nil
	}

	const apiURL = "https://api-v4.mysubmail.com/sms/send"

	signature := s.cfg.SMS.Signature
	if signature == "" {
		signature = "验证码"
	}
	formatted := fmt.Sprintf("【%s】%s", signature, content)

	data := url.Values{}
	data.Set("appid", s.cfg.SMS.AppID)
	data.Set("to", phone)
	data.Set("content", formatted)
	data.Set("sign_type", "normal")
	data.Set("signature", s.cfg.SMS.AppKey)

	req, err := http.NewRequest("POST", apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var result struct {
		Status string `json:"status"`
		Msg    string `json:"msg"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		// 不打印响应体，避免把服务商返回的敏感信息写进日志
		return fmt.Errorf("parse sms response failed")
	}
	if result.Status != "success" {
		return fmt.Errorf("SMS send failed: %s", result.Msg)
	}
	return nil
}

// generateCode 使用 crypto/rand 生成数字验证码。
func (s *SMSService) generateCode() string {
	digits := make([]byte, s.cfg.Code.Length)
	for i := range digits {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			// crypto/rand 失败时退化到时间种子，保证不阻塞登录
			digits[i] = byte(time.Now().UnixNano()%10) + '0'
			continue
		}
		digits[i] = byte(n.Int64()) + '0'
	}
	return string(digits)
}

func isValidPhoneNumber(phone string) bool {
	if len(phone) != 11 || phone[0] != '1' {
		return false
	}
	for _, c := range phone[1:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func maskPhone(phone string) string {
	if len(phone) != 11 {
		return phone
	}
	return phone[:3] + "****" + phone[7:]
}
