-- 已废弃：shard_keys / user_bindings 的表结构统一维护在
--   dtalk/script/mysql/shardkey.sql
-- 首次部署由 dtalk_biz.sql 自动建表；已有库请执行 shardkey.sql。
--
-- 保留本文件只是为了避免旧文档/脚本链接 404。
-- 请不要在这里再维护第二份 DDL，否则字段（douyin_id、is_deleted 等）会漂移。

SELECT 'moved to dtalk/script/mysql/shardkey.sql' AS notice;
