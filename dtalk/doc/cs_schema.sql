-- 客服系统数据库表

-- 客服人员表
-- 每个客服是一个普通 IM 用户，agent_id 就是其 IM 地址
-- App 用户通过公开的客服地址直接加好友进行私聊
-- 客服通过 username/password 登录 Web 工作台，私钥存储在服务端用于签名
CREATE TABLE IF NOT EXISTS cs_agent (
    id              BIGINT       PRIMARY KEY AUTO_INCREMENT,
    agent_id        VARCHAR(64)  NOT NULL UNIQUE COMMENT '客服的 IM 地址(mid)，由私钥派生',
    username        VARCHAR(64)  NOT NULL UNIQUE COMMENT '登录用户名',
    password_hash   VARCHAR(128) NOT NULL        COMMENT 'bcrypt 密码哈希',
    private_key_enc VARCHAR(256) NOT NULL        COMMENT '加密存储的 secp256k1 私钥(hex)，用于服务端代签',
    public_key      VARCHAR(128) NOT NULL        COMMENT '压缩公钥(hex)',
    name            VARCHAR(64)  NOT NULL        COMMENT '客服显示名称，如"客服小美"',
    avatar          VARCHAR(256) DEFAULT ''      COMMENT '头像 URL',
    status          TINYINT      DEFAULT 0       COMMENT '0=离线 1=在线 2=忙碌',
    sort_order      INT          DEFAULT 0       COMMENT '展示排序',
    created_at      TIMESTAMP    DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP    DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='客服人员表';

-- 示例：插入客服（private_key_enc 和 public_key 需用工具提前生成）
-- 密码需用 bcrypt 哈希，私钥建议用 AES 加密后再存储
-- INSERT INTO cs_agent (agent_id, username, password_hash, private_key_enc, public_key, name, avatar, sort_order) VALUES
-- ('1FKxgaEh5fuSm7a35BfUnKYAmradowpiTR', 'cs_xiaomei', '$2a$10$...', 'fa884fd1...', '022db0e0...', '客服小美', '', 1);
