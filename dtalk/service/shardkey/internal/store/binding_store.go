package store

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// BindingRecord 用户账号绑定记录
type BindingRecord struct {
	ID            int64
	Phone         string
	WechatID      string
	XiaohongshuID string
	DouyinID      string
	ChatAddress   string
	IsDeleted     bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// BindingStore 账号绑定存储实现
type BindingStore struct {
	mu sync.RWMutex
	db *sql.DB
}

// NewBindingStore 创建绑定存储
func NewBindingStore(db *sql.DB) *BindingStore {
	return &BindingStore{
		db: db,
	}
}

// BindAccount 绑定账号到聊天地址
// 支持绑定手机号、微信号、小红书账号、抖音号中的一个或多个到同一个 chataddress
// 每个 chataddress 只保留一条绑定记录，重复绑定时更新已有记录
func (s *BindingStore) BindAccount(phone, wechatID, xiaohongshuID, douyinID, chatAddress string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if phone == "" && wechatID == "" && xiaohongshuID == "" && douyinID == "" {
		return fmt.Errorf("至少需要提供一个账号标识（手机号、微信号、小红书账号或抖音号）")
	}

	if chatAddress == "" {
		return fmt.Errorf("聊天地址不能为空")
	}

	// 先查询该 chataddress 是否已有绑定记录
	existing, exists := s.getExistingBinding(chatAddress)

	var err error
	if exists {
		err = s.updateBinding(existing, phone, wechatID, xiaohongshuID, douyinID, chatAddress)
	} else {
		err = s.insertBinding(phone, wechatID, xiaohongshuID, douyinID, chatAddress)
	}
	if err != nil {
		return err
	}
	return s.cleanupOtherBindings(phone, wechatID, xiaohongshuID, douyinID, chatAddress)
}

// cleanupOtherBindings 同一账号标识改绑到新地址后，软删除旧地址上的有效绑定，
// 避免 user_bindings 里同一个手机号/微信号同时存在多条有效记录。
func (s *BindingStore) cleanupOtherBindings(phone, wechatID, xiaohongshuID, douyinID, chatAddress string) error {
	fields := []struct {
		column string
		value  string
	}{
		{"phone", phone},
		{"wechat_id", wechatID},
		{"xiaohongshu_id", xiaohongshuID},
		{"douyin_id", douyinID},
	}
	for _, f := range fields {
		if f.value == "" {
			continue
		}
		query := fmt.Sprintf(`UPDATE user_bindings SET is_deleted = 1, updated_at = NOW()
WHERE %s = ? AND chataddress <> ? AND is_deleted = 0;`, f.column)
		if _, err := s.db.Exec(query, f.value, chatAddress); err != nil {
			return fmt.Errorf("清理旧绑定失败: %w", err)
		}
	}
	return nil
}

// getExistingBinding 查询 chataddress 已有的绑定记录（内部方法，调用方需持有锁）
func (s *BindingStore) getExistingBinding(chatAddress string) (*BindingRecord, bool) {
	query := `SELECT id, phone, wechat_id, xiaohongshu_id, douyin_id FROM user_bindings WHERE chataddress = ? AND is_deleted = 0 LIMIT 1;`

	var id int64
	var phone, wechatID, xiaohongshuID, douyinID sql.NullString
	err := s.db.QueryRow(query, chatAddress).Scan(&id, &phone, &wechatID, &xiaohongshuID, &douyinID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false
		}
		logrus.WithFields(logrus.Fields{
			"chataddress": chatAddress,
			"error":       err.Error(),
		}).Error("Failed to query existing binding")
		return nil, false
	}

	rec := &BindingRecord{
		ID:          id,
		ChatAddress: chatAddress,
	}
	if phone.Valid {
		rec.Phone = phone.String
	}
	if wechatID.Valid {
		rec.WechatID = wechatID.String
	}
	if xiaohongshuID.Valid {
		rec.XiaohongshuID = xiaohongshuID.String
	}
	if douyinID.Valid {
		rec.DouyinID = douyinID.String
	}
	return rec, true
}

// updateBinding 更新已有绑定记录，仅覆盖本次传入的非空字段
func (s *BindingStore) updateBinding(existing *BindingRecord, phone, wechatID, xiaohongshuID, douyinID, chatAddress string) error {
	var setClauses []string
	var args []interface{}

	if phone != "" {
		setClauses = append(setClauses, "phone = ?")
		args = append(args, phone)
	}
	if wechatID != "" {
		setClauses = append(setClauses, "wechat_id = ?")
		args = append(args, wechatID)
	}
	if xiaohongshuID != "" {
		setClauses = append(setClauses, "xiaohongshu_id = ?")
		args = append(args, xiaohongshuID)
	}
	if douyinID != "" {
		setClauses = append(setClauses, "douyin_id = ?")
		args = append(args, douyinID)
	}

	setClauses = append(setClauses, "is_deleted = 0", "updated_at = NOW()")
	args = append(args, existing.ID)

	query := fmt.Sprintf("UPDATE user_bindings SET %s WHERE id = ?;", strings.Join(setClauses, ", "))

	_, err := s.db.Exec(query, args...)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"id":              existing.ID,
			"phone":           phone,
			"wechat_id":       wechatID,
			"xiaohongshu_id":  xiaohongshuID,
			"douyin_id":       douyinID,
			"chataddress":     chatAddress,
			"existing_phone":  existing.Phone,
			"existing_wechat": existing.WechatID,
			"existing_xhs":    existing.XiaohongshuID,
			"existing_douyin": existing.DouyinID,
			"error":           err.Error(),
		}).Error("Failed to update binding")
		return fmt.Errorf("更新绑定账号失败: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"id":             existing.ID,
		"phone":          phone,
		"wechat_id":      wechatID,
		"xiaohongshu_id": xiaohongshuID,
		"douyin_id":      douyinID,
		"chataddress":    chatAddress,
		"old_phone":      existing.Phone,
		"old_wechat":     existing.WechatID,
		"old_xhs":        existing.XiaohongshuID,
		"old_douyin":     existing.DouyinID,
	}).Info("Binding updated successfully")

	return nil
}

// insertBinding 插入新的绑定记录
func (s *BindingStore) insertBinding(phone, wechatID, xiaohongshuID, douyinID, chatAddress string) error {
	query := `INSERT INTO user_bindings (phone, wechat_id, xiaohongshu_id, douyin_id, chataddress, is_deleted, created_at, updated_at) VALUES (?, ?, ?, ?, ?, 0, NOW(), NOW());`

	var phoneVal, wechatVal, xiaohongshuVal, douyinVal interface{}
	if phone != "" {
		phoneVal = phone
	}
	if wechatID != "" {
		wechatVal = wechatID
	}
	if xiaohongshuID != "" {
		xiaohongshuVal = xiaohongshuID
	}
	if douyinID != "" {
		douyinVal = douyinID
	}

	_, err := s.db.Exec(query, phoneVal, wechatVal, xiaohongshuVal, douyinVal, chatAddress)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"phone":          phone,
			"wechat_id":      wechatID,
			"xiaohongshu_id": xiaohongshuID,
			"douyin_id":      douyinID,
			"chataddress":    chatAddress,
			"error":          err.Error(),
		}).Error("Failed to insert binding")
		return fmt.Errorf("绑定账号失败: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"phone":          phone,
		"wechat_id":      wechatID,
		"xiaohongshu_id": xiaohongshuID,
		"douyin_id":      douyinID,
		"chataddress":    chatAddress,
	}).Info("Account bound successfully")

	return nil
}

// GetChatAddressByPhone 根据手机号查询聊天地址
func (s *BindingStore) GetChatAddressByPhone(phone string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
SELECT chataddress 
FROM user_bindings 
WHERE phone = ? AND is_deleted = 0 
ORDER BY updated_at DESC 
LIMIT 1;
`

	var chatAddress string
	err := s.db.QueryRow(query, phone).Scan(&chatAddress)
	if err != nil {
		if err == sql.ErrNoRows {
			logrus.WithFields(logrus.Fields{
				"phone": phone,
			}).Debug("No chat address found for phone")
			return "", false
		}
		logrus.WithFields(logrus.Fields{
			"phone": phone,
			"error": err.Error(),
		}).Error("Failed to query chat address by phone")
		return "", false
	}

	return chatAddress, true
}

// GetChatAddressByWechatID 根据微信号查询聊天地址
func (s *BindingStore) GetChatAddressByWechatID(wechatID string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
SELECT chataddress 
FROM user_bindings 
WHERE wechat_id = ? AND is_deleted = 0 
ORDER BY updated_at DESC 
LIMIT 1;
`

	var chatAddress string
	err := s.db.QueryRow(query, wechatID).Scan(&chatAddress)
	if err != nil {
		if err == sql.ErrNoRows {
			logrus.WithFields(logrus.Fields{
				"wechat_id": wechatID,
			}).Debug("No chat address found for wechat ID")
			return "", false
		}
		logrus.WithFields(logrus.Fields{
			"wechat_id": wechatID,
			"error":     err.Error(),
		}).Error("Failed to query chat address by wechat ID")
		return "", false
	}

	return chatAddress, true
}

// GetChatAddressByXiaohongshuID 根据小红书账号查询聊天地址
func (s *BindingStore) GetChatAddressByXiaohongshuID(xiaohongshuID string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
SELECT chataddress 
FROM user_bindings 
WHERE xiaohongshu_id = ? AND is_deleted = 0 
ORDER BY updated_at DESC 
LIMIT 1;
`

	var chatAddress string
	err := s.db.QueryRow(query, xiaohongshuID).Scan(&chatAddress)
	if err != nil {
		if err == sql.ErrNoRows {
			logrus.WithFields(logrus.Fields{
				"xiaohongshu_id": xiaohongshuID,
			}).Debug("No chat address found for xiaohongshu ID")
			return "", false
		}
		logrus.WithFields(logrus.Fields{
			"xiaohongshu_id": xiaohongshuID,
			"error":          err.Error(),
		}).Error("Failed to query chat address by xiaohongshu ID")
		return "", false
	}

	return chatAddress, true
}

// GetChatAddressByDouyinID 根据抖音号查询聊天地址
func (s *BindingStore) GetChatAddressByDouyinID(douyinID string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
SELECT chataddress 
FROM user_bindings 
WHERE douyin_id = ? AND is_deleted = 0 
ORDER BY updated_at DESC 
LIMIT 1;
`

	var chatAddress string
	err := s.db.QueryRow(query, douyinID).Scan(&chatAddress)
	if err != nil {
		if err == sql.ErrNoRows {
			logrus.WithFields(logrus.Fields{
				"douyin_id": douyinID,
			}).Debug("No chat address found for douyin ID")
			return "", false
		}
		logrus.WithFields(logrus.Fields{
			"douyin_id": douyinID,
			"error":     err.Error(),
		}).Error("Failed to query chat address by douyin ID")
		return "", false
	}

	return chatAddress, true
}

// GetAccountsByChatAddress 根据聊天地址查询绑定的账号信息
// 返回该聊天地址绑定的所有手机号、微信号、小红书账号、抖音号（合并所有记录）
func (s *BindingStore) GetAccountsByChatAddress(chatAddress string) (*BindingRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
SELECT phone, wechat_id, xiaohongshu_id, douyin_id
FROM user_bindings 
WHERE chataddress = ? AND is_deleted = 0;
`

	rows, err := s.db.Query(query, chatAddress)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"chataddress": chatAddress,
			"error":       err.Error(),
		}).Error("Failed to query accounts by chat address")
		return nil, false
	}
	defer rows.Close()

	rec := &BindingRecord{
		ChatAddress: chatAddress,
	}
	found := false

	// 合并所有记录中的账号信息
	for rows.Next() {
		var phone, wechatID, xiaohongshuID, douyinID sql.NullString
		if err := rows.Scan(&phone, &wechatID, &xiaohongshuID, &douyinID); err != nil {
			logrus.WithFields(logrus.Fields{
				"chataddress": chatAddress,
				"error":       err.Error(),
			}).Error("Failed to scan account record")
			continue
		}

		found = true

		// 合并账号信息（保留第一个非空值）
		if phone.Valid && phone.String != "" && rec.Phone == "" {
			rec.Phone = phone.String
		}
		if wechatID.Valid && wechatID.String != "" && rec.WechatID == "" {
			rec.WechatID = wechatID.String
		}
		if xiaohongshuID.Valid && xiaohongshuID.String != "" && rec.XiaohongshuID == "" {
			rec.XiaohongshuID = xiaohongshuID.String
		}
		if douyinID.Valid && douyinID.String != "" && rec.DouyinID == "" {
			rec.DouyinID = douyinID.String
		}
	}

	if err := rows.Err(); err != nil {
		logrus.WithFields(logrus.Fields{
			"chataddress": chatAddress,
			"error":       err.Error(),
		}).Error("Error iterating account records")
		return nil, false
	}

	if !found {
		logrus.WithFields(logrus.Fields{
			"chataddress": chatAddress,
		}).Debug("No accounts found for chat address")
		return nil, false
	}

	return rec, true
}

// DeleteBinding 软删除绑定关系
func (s *BindingStore) DeleteBinding(chatAddress string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
UPDATE user_bindings 
SET is_deleted = 1, updated_at = NOW() 
WHERE chataddress = ? AND is_deleted = 0;
`

	result, err := s.db.Exec(query, chatAddress)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"chataddress": chatAddress,
			"error":       err.Error(),
		}).Error("Failed to delete binding")
		return fmt.Errorf("删除绑定关系失败: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"chataddress": chatAddress,
			"error":       err.Error(),
		}).Error("Failed to get rows affected")
		return fmt.Errorf("获取影响行数失败: %w", err)
	}

	if rowsAffected == 0 {
		logrus.WithFields(logrus.Fields{
			"chataddress": chatAddress,
		}).Warn("No binding found to delete")
		return fmt.Errorf("未找到可删除的绑定关系")
	}

	logrus.WithFields(logrus.Fields{
		"chataddress":   chatAddress,
		"rows_affected": rowsAffected,
	}).Info("Binding deleted successfully")

	return nil
}
