####  获取模块启用状态
URL: /app/modules/all

`post`

**请求参数：**

无

**返回参数：**

| **参数**    | **名字**     | **类型** | **说明**   |
| ----------- | ------------ | -------- | ---------- |
| name | 模块名称 | string   | 钱包模块：wallet，企业模块：oa，红包模块：redpacket |
| isEnabled | 是否启用 | bool |               |
| endPoints | 模块访问地址 | string | |

```json
{
    "result": 0,
    "message": "",
    "data": [
        {
            "name": "wallet",
            "isEnabled": true,
            "endPoints": [
                "https://127.0.0.1:8083"
            ]
        },
        {
            "name": "oa",
            "isEnabled": true,
            "endPoints": [
                "http://127.0.0.1"
            ]
        }
    ]
}
```

#### 消息撤回
URL: /app/record/revoke

`post`
**请求参数：**

| **参数**    | **名字**     | **类型** | **约束**   | **说明**   |
| ----------- | ------------ | -------- | ---------- |---------- |
| type | 类型 | int  | 必填 | 0->撤回私聊消息；1->撤回群聊消息 |
| logId | 消息id | int64 | 必填|               |

```json
{
    "type": 0,
    "logId": 0
}
```

**返回参数：**
无
```json
{
    "result": 0,
    "message": "",
    "data": {}
}
```

#### 批量撤回消息
URL: /app/record/revoke-batch

`post`

**请求参数：**

| **参数**    | **名字**     | **类型** | **约束**   | **说明**   |
| ----------- | ------------ | -------- | ---------- |---------- |
| type | 类型 | int  | 必填 | 0->撤回私聊消息；1->撤回群聊消息 |
| logIds | 消息id列表 | int64[] | 必填，1~50条 | 自动去重，须属于同一会话 |

```json
{
    "type": 1,
    "logIds": [100001, 100002, 100003]
}
```

**返回参数：**

| **参数**    | **名字**     | **类型** | **说明**   |
| ----------- | ------------ | -------- | ---------- |
| total | 总数 | int | 去重后的请求条数 |
| successCount | 成功数 | int | |
| failedCount | 失败数 | int | |
| succeeded | 成功列表 | int64[] | 成功的 logId |
| failed | 失败明细 | object[] | 含 logId、reason |

```json
{
    "result": 0,
    "message": "",
    "data": {
        "total": 3,
        "successCount": 2,
        "failedCount": 1,
        "succeeded": [100001, 100002],
        "failed": [
            {
                "logId": 100003,
                "reason": "权限不足"
            }
        ]
    }
}
```

**说明：**
- 支持部分成功，单条失败不会导致整批 result 非 0
- 权限规则与单条撤回 `/app/record/revoke` 一致

#### 关注消息
URL: /app/record/focus

`post` `auth`  
**请求参数：**

| **参数**    | **名字**     | **类型** | **约束**   | **说明**   |
| ----------- | ------------ | -------- | ---------- |---------- |
| type | 类型 | int  | 必填 | 0->关注私聊消息；1->关注群聊消息 |
| logId | 消息id | int64 | 必填|               |

```json
{
    "type": 0,
    "logId": 0
}
```

**返回参数：**
无
```json
{
    "result": 0,
    "message": "",
    "data": {}
}
```
