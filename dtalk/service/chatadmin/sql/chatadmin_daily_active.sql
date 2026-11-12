-- chatadmin 每日活跃快照表（dtalk 库）
-- 用于沉淀日活用户 / 日活群组，供运营后台趋势查询

CREATE TABLE IF NOT EXISTS `chatadmin_daily_active` (
  `id` BIGINT PRIMARY KEY AUTO_INCREMENT,
  `stat_date` DATE NOT NULL COMMENT '统计日 YYYY-MM-DD（本地日历日）',
  `active_users` BIGINT NOT NULL DEFAULT 0 COMMENT '日活用户数（去重）',
  `active_groups` BIGINT NOT NULL DEFAULT 0 COMMENT '日活群组数（去重）',
  `total_users` BIGINT NOT NULL DEFAULT 0 COMMENT '快照时总用户数',
  `total_groups` BIGINT NOT NULL DEFAULT 0 COMMENT '快照时总群组数',
  `created_at` BIGINT NOT NULL COMMENT '创建时间(ms)',
  `updated_at` BIGINT NOT NULL COMMENT '更新时间(ms)',
  UNIQUE KEY `uk_stat_date` (`stat_date`),
  KEY `idx_stat_date` (`stat_date`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='chatadmin 每日活跃入库记录';
