-- 活跃统计按天检索。chatadmin 启动后也会在后台补这两枚索引（已存在则跳过）。
-- 在线添加，不锁写入。大表第一次会比较久。

ALTER TABLE dtalk_signal_content
  ADD INDEX idx_signal_create_uid (create_time, uid),
  ALGORITHM=INPLACE, LOCK=NONE;

ALTER TABLE dtalk_group_msg_relation
  ADD INDEX idx_group_msg_type_time_other (type, create_time, other_uid),
  ALGORITHM=INPLACE, LOCK=NONE;
