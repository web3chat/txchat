# CS Agent 管理工具

用于生成客服账号数据并输出可直接执行的 SQL INSERT 语句。

## 功能

- 自动生成或导入 secp256k1 密钥对
- 从公钥派生 IM 地址（与 dtalk 用户体系一致）
- 使用 bcrypt 对密码进行哈希
- 使用 AES-256-GCM 加密私钥（可选，推荐生产环境使用）
- 输出完整的 `cs_agent` 表 INSERT 语句

## 编译

```bash
cd dtalk/service/cs/tools
go build -o cs-tool .
```

## 使用方式

### 方式一：命令行参数（适合脚本/自动化）

```bash
# 自动生成密钥对，明文存储私钥（开发环境）
./cs-tool -username cs_xiaomei -password "abc123" -name "客服小美"

# 自动生成密钥对，AES 加密私钥（生产环境）
./cs-tool -username cs_xiaomei -password "abc123" -name "客服小美" \
  -aes-key "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

# 导入已有私钥
./cs-tool -username cs_xiaoming -password "pwd456" -name "客服小明" \
  -private-key "fa884fd1b47d9e9e8dc19e47dc1a794a524ce5d4ee1b82ec92b1ffc1f109c2b6"

# 设置头像和排序
./cs-tool -username cs_xiaohong -password "pwd789" -name "客服小红" \
  -avatar "https://example.com/avatar3.png" -sort 3
```

### 方式二：交互式输入（适合手动操作）

```bash
./cs-tool
```

程序会依次提示输入用户名、密码、显示名称。

### 仅生成密钥对

```bash
./cs-tool -gen-key
```

输出示例：

```
新生成的 secp256k1 密钥对:
  私钥: fa884fd1b47d9e9e8dc19e47dc1a794a524ce5d4ee1b82ec92b1ffc1f109c2b6
  公钥: 022db0e08669b30c5dab8c564b428db4944912144088943ec9b690a9046bc8f78b
  地址: 1FKxgaEh5fuSm7a35BfUnKYAmradowpiTR
```

## 参数说明

| 参数 | 必填 | 说明 |
|------|------|------|
| `-username` | 是 | 客服登录用户名 |
| `-password` | 是 | 客服登录密码 |
| `-name` | 是 | 客服显示名称（如"客服小美"） |
| `-avatar` | 否 | 头像 URL |
| `-sort` | 否 | 展示排序，默认 0 |
| `-private-key` | 否 | 已有的 secp256k1 私钥（hex），不提供则自动生成 |
| `-aes-key` | 否 | AES-256 加密密钥（hex，64字符=32字节），用于加密存储私钥 |
| `-gen-key` | 否 | 仅生成密钥对并打印，不生成 SQL |

## 输出示例

```
======================================================================
  客服账号信息
======================================================================
  用户名:        cs_xiaomei
  显示名称:      客服小美
  IM 地址:       1FKxgaEh5fuSm7a35BfUnKYAmradowpiTR
  公钥:          022db0e08669b30c5dab8c564b428db4944912144088943ec9b690a9046bc8f78b
  私钥(请妥善保存): fa884fd1b47d9e9e8dc19e47dc1a794a524ce5d4ee1b82ec92b1ffc1f109c2b6
======================================================================

-- 复制以下 SQL 插入到数据库:
INSERT INTO cs_agent (agent_id, username, password_hash, private_key_enc, public_key, name, avatar, status, sort_order)
VALUES ('1FKxgaEh5fuSm7a35BfUnKYAmradowpiTR', 'cs_xiaomei', '$2a$10$...', 'fa884fd1...', '022db0e0...', '客服小美', '', 0, 0);
```

## 关于 AES 密钥

AES-256 密钥需要 32 字节（64 个十六进制字符）。可以用以下方式生成：

```bash
# Linux/macOS
openssl rand -hex 32

# 输出示例: 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
```

生产环境中，此密钥应配置在 CS 服务的配置文件中（`cs.toml`），用于运行时解密私钥签发 FZM-SIGNATURE。

## 快速上手示例

```bash
# 1. 编译工具
go build -o cs-tool .

# 2. 生成 AES 密钥（生产环境）
AES_KEY=$(openssl rand -hex 32)
echo "AES 密钥: $AES_KEY （请保存到 cs.toml 配置中）"

# 3. 创建三个客服账号
./cs-tool -username cs_agent1 -password "pass1" -name "客服小美" -sort 1 -aes-key "$AES_KEY"
./cs-tool -username cs_agent2 -password "pass2" -name "客服小明" -sort 2 -aes-key "$AES_KEY"
./cs-tool -username cs_agent3 -password "pass3" -name "客服小红" -sort 3 -aes-key "$AES_KEY"

# 4. 将输出的 SQL 语句执行到数据库中
```
