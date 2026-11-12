-- 运营台操作留痕，建在 dtalk_record。
-- chatadmin 启动时也会执行同等的 CREATE TABLE IF NOT EXISTS。

CREATE TABLE IF NOT EXISTS dtalk_admin_audit (
  id          BIGINT       NOT NULL AUTO_INCREMENT,
  operator    VARCHAR(128) NOT NULL DEFAULT '' COMMENT '操作人地址',
  action      VARCHAR(64)  NOT NULL COMMENT '动作',
  target_type VARCHAR(32)  NOT NULL DEFAULT '' COMMENT '对象类型',
  target_id   VARCHAR(128) NOT NULL DEFAULT '' COMMENT '对象 ID',
  extra       VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '补充说明',
  client_ip   VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '来源 IP',
  created_at  BIGINT       NOT NULL COMMENT '操作时间，Unix 毫秒',
  PRIMARY KEY (id),
  KEY idx_dtalk_admin_audit_created (created_at),
  KEY idx_dtalk_admin_audit_action (action),
  KEY idx_dtalk_admin_audit_target (target_type, target_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='运营台操作留痕';
