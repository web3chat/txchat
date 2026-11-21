-- shardkey 分片密钥服务表结构（canonical）
-- 首次部署：由 dtalk_biz.sql / MySQL initdb 自动执行
-- 已有库：手工执行本文件（全部 CREATE TABLE IF NOT EXISTS，可重复执行）
--
-- 注意：分片密钥由服务使用 AES-256-GCM 加密后落库，本表不保存明文。

CREATE TABLE IF NOT EXISTS shard_keys (
  id                    BIGINT PRIMARY KEY AUTO_INCREMENT,
  phone                 VARCHAR(32)   NOT NULL,
  chataddress           VARCHAR(128)  NOT NULL,
  shard_key             VARCHAR(512)  NOT NULL COMMENT '加密后的当前分片密钥',
  previous_shard_key    VARCHAR(512)  NULL COMMENT '加密后的上一个分片密钥（历史追溯）',
  previous_chataddress  VARCHAR(128)  NULL COMMENT '上一个聊天地址（历史追溯）',
  is_updated            TINYINT(1)    NOT NULL DEFAULT 0 COMMENT '是否被更新过，0=未更新，1=已更新',
  version               INT           NOT NULL DEFAULT 1 COMMENT '密钥版本号，每次重置递增',
  is_deleted            TINYINT(1)    NOT NULL DEFAULT 0 COMMENT '是否已注销，0=正常，1=已注销',
  created_at            TIMESTAMP     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at            TIMESTAMP     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uq_phone_chataddress (phone, chataddress),
  KEY idx_phone (phone),
  KEY idx_chataddress (chataddress),
  KEY idx_is_updated (is_updated),
  KEY idx_version (version),
  KEY idx_is_deleted (is_deleted)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS user_bindings (
  id                  BIGINT PRIMARY KEY AUTO_INCREMENT,
  phone               VARCHAR(32)   NULL COMMENT '手机号',
  wechat_id           VARCHAR(128)  NULL COMMENT '微信号',
  xiaohongshu_id      VARCHAR(128)  NULL COMMENT '小红书账号',
  douyin_id           VARCHAR(128)  NULL COMMENT '抖音号',
  chataddress         VARCHAR(128)  NOT NULL COMMENT '聊天地址',
  is_deleted          TINYINT(1)    NOT NULL DEFAULT 0 COMMENT '是否已删除，0=正常，1=已删除',
  created_at          TIMESTAMP     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at          TIMESTAMP     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uq_phone_chataddress (phone, chataddress),
  UNIQUE KEY uq_wechat_chataddress (wechat_id, chataddress),
  UNIQUE KEY uq_xiaohongshu_chataddress (xiaohongshu_id, chataddress),
  UNIQUE KEY uq_douyin_chataddress (douyin_id, chataddress),
  KEY idx_phone (phone),
  KEY idx_wechat_id (wechat_id),
  KEY idx_xiaohongshu_id (xiaohongshu_id),
  KEY idx_douyin_id (douyin_id),
  KEY idx_chataddress (chataddress),
  KEY idx_is_deleted (is_deleted)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- ============================================================
-- 已有库升级（如果列/索引已存在，执行会报错，可逐条忽略或用 mysql --force）
-- 新库由上面的 CREATE TABLE IF NOT EXISTS 建全，以下语句只对老表生效。
-- ============================================================

ALTER TABLE shard_keys
  ADD COLUMN previous_shard_key VARCHAR(512) NULL COMMENT '加密后的上一个分片密钥（历史追溯）' AFTER shard_key;
ALTER TABLE shard_keys
  ADD COLUMN previous_chataddress VARCHAR(128) NULL COMMENT '上一个聊天地址（历史追溯）' AFTER previous_shard_key;
ALTER TABLE shard_keys
  ADD COLUMN is_updated TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否被更新过，0=未更新，1=已更新' AFTER previous_chataddress;
ALTER TABLE shard_keys
  ADD COLUMN version INT NOT NULL DEFAULT 1 COMMENT '密钥版本号，每次重置递增' AFTER is_updated;
ALTER TABLE shard_keys
  ADD COLUMN is_deleted TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否已注销，0=正常，1=已注销' AFTER version;

ALTER TABLE user_bindings
  ADD COLUMN douyin_id VARCHAR(128) NULL COMMENT '抖音号';
ALTER TABLE user_bindings
  ADD UNIQUE KEY uq_douyin_chataddress (douyin_id, chataddress);

-- 说明：
-- 1. MySQL 不支持 ADD COLUMN IF NOT EXISTS，重复执行会报 Duplicate column/key；
--    全新库这些 ALTER 都会报错，属正常现象，CREATE 已经建好。
-- 2. 升级已有库建议：mysql --force -h <host> -u root -p dtalk < shardkey.sql
