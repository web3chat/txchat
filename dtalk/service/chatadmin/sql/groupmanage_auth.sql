-- groupmanage 模块 - 管理员白名单与登录nonce建表脚本
-- 执行前请确保已选择正确数据库：USE dtalk;

-- 管理员白名单表：允许登录的以太坊地址
CREATE TABLE IF NOT EXISTS `groupmanage_admin_whitelist` (
  `id` BIGINT PRIMARY KEY AUTO_INCREMENT,
  `address` VARCHAR(64) NOT NULL UNIQUE COMMENT '以太坊地址(小写, 0x...)',
  `remark` VARCHAR(255) DEFAULT '' COMMENT '备注',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '0=禁用, 1=启用',
  `level` TINYINT NOT NULL DEFAULT 0 COMMENT '管理员级别: 0=普通管理员, 1=超级管理员',
  `created_by` VARCHAR(64) DEFAULT '' COMMENT '创建者',
  `created_at` BIGINT NOT NULL COMMENT '创建时间(ms)',
  `updated_at` BIGINT NOT NULL COMMENT '更新时间(ms)'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 登录一次性随机串表：nonce
CREATE TABLE IF NOT EXISTS `groupmanage_auth_nonce` (
  `address` VARCHAR(64) PRIMARY KEY COMMENT '以太坊地址(小写, 0x...)',
  `nonce` VARCHAR(128) NOT NULL COMMENT '一次性随机串',
  `expire_at` BIGINT NOT NULL COMMENT '过期时间(ms)'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 索引优化（如需按状态查询白名单）
CREATE INDEX IF NOT EXISTS `idx_whitelist_status` ON `groupmanage_admin_whitelist`(`status`);

-- 如果表已存在，添加level字段（用于升级已有数据库）
-- 注意：如果level字段已存在，执行此语句会报错，可以忽略或先检查字段是否存在
-- 检查字段是否存在的方法：
-- SELECT COUNT(*) FROM information_schema.COLUMNS 
-- WHERE TABLE_SCHEMA = 'dtalk' AND TABLE_NAME = 'groupmanage_admin_whitelist' AND COLUMN_NAME = 'level';
-- 如果返回0，则执行下面的ALTER TABLE语句
ALTER TABLE `groupmanage_admin_whitelist` 
ADD COLUMN `level` TINYINT NOT NULL DEFAULT 0 COMMENT '管理员级别: 0=普通管理员, 1=超级管理员' AFTER `status`;


-- 示例数据：白名单允许登录的以太坊地址（均为小写）
-- 注意：请替换为你自己的管理员地址
INSERT INTO `groupmanage_admin_whitelist` (`address`, `remark`, `status`, `level`, `created_by`, `created_at`, `updated_at`) VALUES
('0x0B9bDAA14a81d52d2EbFfF3EE1c627F536fB4aBF', '示例超级管理员A', 1, 1, 'init', UNIX_TIMESTAMP()*1000, UNIX_TIMESTAMP()*1000),
('0xdd137b41B5e39F68e55C424dE04202cA04e9FE56', '示例普通管理员B', 1, 0, 'init', UNIX_TIMESTAMP()*1000, UNIX_TIMESTAMP()*1000),
('0xbe428900087e2f6dbbafb36e2f7785abaff5ff90', '示例普通管理员C', 1, 0, 'init', UNIX_TIMESTAMP()*1000, UNIX_TIMESTAMP()*1000);


