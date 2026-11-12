-- 创建群组白名单表
CREATE TABLE IF NOT EXISTS `group_create_whitelist` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `address` varchar(64) NOT NULL COMMENT '用户地址',
  `remark` varchar(255) DEFAULT NULL COMMENT '备注说明',
  `status` tinyint(4) NOT NULL DEFAULT '1' COMMENT '状态：1=启用，0=禁用',
  `max_members` int(11) NOT NULL DEFAULT 500 COMMENT '群聊最大人数上限',
  `created_by` varchar(64) DEFAULT NULL COMMENT '创建人地址',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_address` (`address`),
  KEY `idx_status` (`status`),
  KEY `idx_max_members` (`max_members`),
  KEY `idx_created_at` (`created_at`),
  KEY `idx_status_max_members` (`status`, `max_members`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='群组创建白名单表';

-- 插入示例数据
INSERT INTO `group_create_whitelist` (`address`, `remark`, `status`, `max_members`, `created_by`) VALUES
('1Lf4oAVkZihvaXJW23RhwnDuMTeJjNkqh5', '管理员账户', 1, 2000, 'system'),
('1A2B3C4D5E6F7G8H9I0J1K2L3M4N5O6P7Q8R', 'VIP用户', 1, 1000, 'system'),
('1Z9Y8X7W6V5U4T3S2R1Q0P9O8N7M6L5K4J3I2H1G', '普通用户', 1, 500, 'system');
