-- 为白名单表添加 max_groups 字段（最大创建群聊个数）
ALTER TABLE `group_create_whitelist` 
ADD COLUMN `max_groups` int(11) NOT NULL DEFAULT 10 COMMENT '最大创建群聊个数' AFTER `max_members`;

-- 为现有数据设置默认值（如果已有数据）
UPDATE `group_create_whitelist` SET `max_groups` = 10 WHERE `max_groups` IS NULL OR `max_groups` = 0;

