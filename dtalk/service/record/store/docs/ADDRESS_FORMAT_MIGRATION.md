# 地址格式兼容性迁移说明

## 问题描述

当使用 **1x地址（Base58格式）** 与 **0x地址（以太坊格式）** 通信时，出现数据库错误：

```
Error 1406: Data too long for column 'receiver_id' at row 1
```

### 根本原因

- **Base58格式地址**（1x开头）：通常约 **34字符**，例如：`19BKdzKcQbJ7N6q7Ys3KYaJHVSnCF3raAo`
- **以太坊0x格式地址**：固定 **42字符**（0x + 40个十六进制字符），例如：`0xa100b68ec7df4562457dc813f52f8a1a69ba4e01`

原数据库表设计中，所有地址相关字段都是 `varchar(40)`，无法存储42字符的0x地址。

## 影响范围

以下表的所有地址相关字段需要修改：

1. **dtalk_msg_content**
   - `sender_id` varchar(40) → varchar(50)
   - `receiver_id` varchar(40) → varchar(50)

2. **dtalk_group_msg_content**
   - `sender_id` varchar(40) → varchar(50)
   - `receiver_id` varchar(40) → varchar(50)

3. **dtalk_msg_relation**
   - `owner_uid` varchar(40) → varchar(50)
   - `other_uid` varchar(40) → varchar(50)

4. **dtalk_group_msg_relation**
   - `owner_uid` varchar(40) → varchar(50)
   - `other_uid` varchar(40) → varchar(50)

5. **dtalk_msg_version**
   - `uid` varchar(40) → varchar(50)

6. **dtalk_signal_content**
   - `uid` varchar(40) → varchar(50)

## 解决方案

### 执行迁移脚本

执行以下SQL脚本进行数据库迁移：

```bash
mysql -u用户名 -p密码 dtalk_record < dtalk/script/mysql/dtalk_record_migration_address_length.sql
```

或者直接在MySQL客户端执行：

```sql
source dtalk/script/mysql/dtalk_record_migration_address_length.sql;
```

### 迁移脚本内容

迁移脚本会将所有地址相关字段从 `varchar(40)` 扩展到 `varchar(50)`，确保：
- ✅ 兼容Base58格式地址（34字符）
- ✅ 兼容以太坊0x格式地址（42字符）
- ✅ 保留足够的扩展空间（50字符）

### 验证迁移

执行迁移后，可以运行以下SQL验证：

```sql
SELECT 
  TABLE_NAME,
  COLUMN_NAME,
  DATA_TYPE,
  CHARACTER_MAXIMUM_LENGTH
FROM 
  INFORMATION_SCHEMA.COLUMNS
WHERE 
  TABLE_SCHEMA = 'dtalk_record'
  AND COLUMN_NAME IN ('sender_id', 'receiver_id', 'owner_uid', 'other_uid', 'uid')
ORDER BY 
  TABLE_NAME, COLUMN_NAME;
```

所有字段的 `CHARACTER_MAXIMUM_LENGTH` 应该显示为 `50`。

## 注意事项

1. **备份数据**：执行迁移前，请务必备份数据库
2. **停机时间**：ALTER TABLE 操作可能需要锁表，建议在低峰期执行
3. **索引影响**：字段长度修改不会影响现有索引，但建议验证索引是否正常
4. **应用兼容性**：代码层面无需修改，Go代码中的 `string` 类型可以存储任意长度的地址

## 测试验证

迁移完成后，测试以下场景：

1. ✅ 1x地址与1x地址通信（原有功能）
2. ✅ 0x地址与0x地址通信（新增功能）
3. ✅ 1x地址与0x地址通信（混合场景，之前会报错）

## 相关文件

- 迁移脚本：`dtalk/script/mysql/dtalk_record_migration_address_length.sql`
- 原始表结构：`dtalk/script/mysql/dtalk_record.sql`
- 代码实现：`dtalk/service/record/store/dao/db.go`







