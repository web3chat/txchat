-- 分片密钥表，用于存储手机号、地址和分片密钥
-- 适用于 MySQL（InnoDB），如使用 PostgreSQL 请调整自增/时间默认值

CREATE TABLE IF NOT EXISTS shard_keys (
  id                  BIGINT PRIMARY KEY AUTO_INCREMENT,
  phone               VARCHAR(32)   NOT NULL,
  chataddress         VARCHAR(128)  NOT NULL,
  shard_key           VARCHAR(512)  NOT NULL COMMENT '加密后的当前分片密钥',
  previous_shard_key  VARCHAR(512)  NULL COMMENT '加密后的上一个分片密钥（历史追溯）',
  previous_chataddress VARCHAR(128) NULL COMMENT '上一个聊天地址（历史追溯）',
  is_updated          TINYINT(1)    NOT NULL DEFAULT 0 COMMENT '是否被更新过，0=未更新，1=已更新',
  version             INT           NOT NULL DEFAULT 1 COMMENT '密钥版本号，每次重置递增',
  is_deleted          TINYINT(1)    NOT NULL DEFAULT 0 COMMENT '是否已注销，0=正常，1=已注销',
  created_at          TIMESTAMP     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at          TIMESTAMP     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uq_phone_chataddress (phone, chataddress),
  KEY idx_phone (phone),
  KEY idx_chataddress (chataddress),
  KEY idx_is_updated (is_updated),
  KEY idx_version (version),
  KEY idx_is_deleted (is_deleted)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 如果表已存在，添加新字段的迁移脚本
-- 注意：MySQL 不支持 IF NOT EXISTS，需要分别执行
-- 如果字段已存在，执行会报错，可以忽略错误或手动检查

-- 修改 shard_key 字段长度（如果字段已存在，会成功执行）
ALTER TABLE shard_keys 
  MODIFY COLUMN shard_key VARCHAR(512) NOT NULL COMMENT '加密后的当前分片密钥';

-- 添加 previous_shard_key 字段（如果已存在会报错，可忽略）
ALTER TABLE shard_keys 
  ADD COLUMN previous_shard_key VARCHAR(512) NULL COMMENT '加密后的上一个分片密钥（历史追溯）' AFTER shard_key;

-- 添加 previous_chataddress 字段（如果已存在会报错，可忽略）
ALTER TABLE shard_keys 
  ADD COLUMN previous_chataddress VARCHAR(128) NULL COMMENT '上一个聊天地址（历史追溯）' AFTER previous_shard_key;

-- 添加 is_updated 字段（如果已存在会报错，可忽略）
ALTER TABLE shard_keys 
  ADD COLUMN is_updated TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否被更新过，0=未更新，1=已更新' AFTER previous_chataddress;

-- 添加 version 字段（如果已存在会报错，可忽略）
ALTER TABLE shard_keys 
  ADD COLUMN version INT NOT NULL DEFAULT 1 COMMENT '密钥版本号，每次重置递增' AFTER is_updated;

-- 添加索引（如果已存在会报错，可忽略）
ALTER TABLE shard_keys 
  ADD INDEX idx_is_updated (is_updated);

ALTER TABLE shard_keys 
  ADD INDEX idx_version (version);

-- 添加 is_deleted 字段（如果已存在会报错，可忽略）
-- 用于软删除/注销功能，0=正常，1=已注销
ALTER TABLE shard_keys 
  ADD COLUMN is_deleted TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否已注销，0=正常，1=已注销' AFTER version;

-- 添加索引（如果已存在会报错，可忽略）
ALTER TABLE shard_keys 
  ADD INDEX idx_is_deleted (is_deleted);

-- ============================================
-- 用户账号绑定表，用于存储手机号、微信号、小红书账号与聊天地址的绑定关系
-- ============================================

CREATE TABLE IF NOT EXISTS user_bindings (
  id                  BIGINT PRIMARY KEY AUTO_INCREMENT,
  phone               VARCHAR(32)   NULL COMMENT '手机号',
  wechat_id           VARCHAR(128)  NULL COMMENT '微信号',
  xiaohongshu_id      VARCHAR(128)  NULL COMMENT '小红书账号',
  chataddress         VARCHAR(128)  NOT NULL COMMENT '聊天地址',
  is_deleted          TINYINT(1)    NOT NULL DEFAULT 0 COMMENT '是否已删除，0=正常，1=已删除',
  created_at          TIMESTAMP     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at          TIMESTAMP     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uq_phone_chataddress (phone, chataddress),
  UNIQUE KEY uq_wechat_chataddress (wechat_id, chataddress),
  UNIQUE KEY uq_xiaohongshu_chataddress (xiaohongshu_id, chataddress),
  KEY idx_phone (phone),
  KEY idx_wechat_id (wechat_id),
  KEY idx_xiaohongshu_id (xiaohongshu_id),
  KEY idx_chataddress (chataddress),
  KEY idx_is_deleted (is_deleted)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 添加索引（如果已存在会报错，可忽略）
ALTER TABLE user_bindings 
  ADD INDEX idx_phone (phone);

ALTER TABLE user_bindings 
  ADD INDEX idx_wechat_id (wechat_id);

ALTER TABLE user_bindings 
  ADD INDEX idx_xiaohongshu_id (xiaohongshu_id);

ALTER TABLE user_bindings 
  ADD INDEX idx_chataddress (chataddress);

ALTER TABLE user_bindings 
  ADD INDEX idx_is_deleted (is_deleted);
