package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/web3chat/txchat/dtalk/service/shardkey/config"
	"github.com/web3chat/txchat/dtalk/service/shardkey/internal/util"
)

// ShardRecord 分片密钥记录。
type ShardRecord struct {
	Phone               string
	ChatAddress         string
	ShardKey            string // 解密后的当前密钥
	PreviousShardKey    string // 解密后的上一个密钥（历史追溯）
	PreviousChatAddress string // 上一个聊天地址（历史追溯）
	IsUpdated           bool
	Version             int
	IsDeleted           bool
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// ShardStore 分片密钥存储。
type ShardStore struct {
	db         *sql.DB
	encryptKey string
}

func NewShardStore(db *sql.DB, cfg *config.Config) *ShardStore {
	return &ShardStore{
		db:         db,
		encryptKey: cfg.Security.EncryptKey,
	}
}

// Save 首次保存分片密钥（is_updated=0）。
func (s *ShardStore) Save(phone, chatAddress, shardKey string) error {
	return s.save(phone, chatAddress, shardKey, false)
}

// Reset 重置分片密钥：无论地址是否变化，都把该手机号的当前记录更新为新地址/新密钥，
// 旧地址记录软删除，并保留 previous_* 供追溯。
func (s *ShardStore) Reset(phone, chatAddress, shardKey string) error {
	return s.save(phone, chatAddress, shardKey, true)
}

func (s *ShardStore) save(phone, chatAddress, shardKey string, isReset bool) error {
	encryptedKey, err := util.Encrypt(shardKey, s.encryptKey)
	if err != nil {
		return fmt.Errorf("加密密钥失败: %w", err)
	}

	if !isReset {
		_, err := s.db.Exec(`
INSERT INTO shard_keys (phone, chataddress, shard_key, is_updated, version, created_at, updated_at)
VALUES (?, ?, ?, 0, 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE shard_key=VALUES(shard_key), is_updated=0, is_deleted=0, updated_at=NOW();
`, phone, chatAddress, encryptedKey)
		return err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 取该手机号当前的有效记录（不按地址，地址可能已变化）
	var (
		currentVersion      int
		currentEncryptedKey string
		currentChatAddress  string
	)
	err = tx.QueryRow(`
SELECT version, shard_key, chataddress
FROM shard_keys
WHERE phone = ? AND is_deleted = 0
ORDER BY updated_at DESC
LIMIT 1
FOR UPDATE
`, phone).Scan(&currentVersion, &currentEncryptedKey, &currentChatAddress)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	version := 1
	var previousEncryptedKey interface{}
	var previousChatAddr interface{}
	if err == nil {
		version = currentVersion + 1
		previousEncryptedKey = currentEncryptedKey
		previousChatAddr = currentChatAddress
	}

	_, err = tx.Exec(`
INSERT INTO shard_keys (phone, chataddress, shard_key, previous_shard_key, previous_chataddress, is_updated, version, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, 1, ?, NOW(), NOW())
ON DUPLICATE KEY UPDATE
  previous_shard_key=VALUES(previous_shard_key),
  previous_chataddress=VALUES(previous_chataddress),
  shard_key=VALUES(shard_key),
  is_updated=1,
  version=VALUES(version),
  is_deleted=0,
  updated_at=NOW();
`, phone, chatAddress, encryptedKey, previousEncryptedKey, previousChatAddr, version)
	if err != nil {
		return err
	}

	// 地址变化时，旧地址记录软删除，避免同一手机号多条有效记录
	if currentChatAddress != "" && currentChatAddress != chatAddress {
		if _, err := tx.Exec(`
UPDATE shard_keys SET is_deleted = 1, updated_at = NOW()
WHERE phone = ? AND chataddress <> ? AND is_deleted = 0;
`, phone, chatAddress); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetByPhone 返回手机号对应的最新有效分片密钥。
func (s *ShardStore) GetByPhone(phone string) (*ShardRecord, bool) {
	query := `
SELECT phone, chataddress, shard_key, previous_shard_key, previous_chataddress, is_updated, version, is_deleted, created_at, updated_at
FROM shard_keys
WHERE phone = ? AND is_deleted = 0
ORDER BY updated_at DESC
LIMIT 1;
`
	row := s.db.QueryRow(query, phone)
	rec := &ShardRecord{}
	var encryptedKey string
	var previousEncryptedKey sql.NullString
	var previousChatAddr sql.NullString
	var isUpdatedInt int
	var isDeletedInt int
	if err := row.Scan(&rec.Phone, &rec.ChatAddress, &encryptedKey, &previousEncryptedKey, &previousChatAddr, &isUpdatedInt, &rec.Version, &isDeletedInt, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
		if err != sql.ErrNoRows {
			logrus.WithError(err).WithField("phone", maskPhone(phone)).Error("query shard key failed")
		}
		return nil, false
	}

	decryptedKey, err := util.Decrypt(encryptedKey, s.encryptKey)
	if err != nil {
		// 兼容历史未加密数据；新数据必须加密
		logrus.WithError(err).WithField("phone", maskPhone(phone)).Warn("decrypt shard key failed, fallback to raw value")
		rec.ShardKey = encryptedKey
	} else {
		rec.ShardKey = decryptedKey
	}

	if previousEncryptedKey.Valid && previousEncryptedKey.String != "" {
		if decrypted, err := util.Decrypt(previousEncryptedKey.String, s.encryptKey); err == nil {
			rec.PreviousShardKey = decrypted
		}
	}
	if previousChatAddr.Valid {
		rec.PreviousChatAddress = previousChatAddr.String
	}
	rec.IsUpdated = isUpdatedInt == 1
	rec.IsDeleted = isDeletedInt == 1
	return rec, true
}

// Delete 软删除（注销）指定手机号的分片密钥。
func (s *ShardStore) Delete(phone string) error {
	result, err := s.db.Exec(`
UPDATE shard_keys SET is_deleted = 1, updated_at = NOW()
WHERE phone = ? AND is_deleted = 0;
`, phone)
	if err != nil {
		return fmt.Errorf("删除分片密钥失败: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("未找到可注销的分片密钥记录")
	}
	return nil
}

func maskPhone(phone string) string {
	if len(phone) != 11 {
		return phone
	}
	return phone[:3] + "****" + phone[7:]
}
