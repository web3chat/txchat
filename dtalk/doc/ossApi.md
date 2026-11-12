# 对象存储服务
### 获取token
URL: /oss/get-token

`get`|`post`

**请求参数：**  
无

**返回参数：**  

```json
{
    "result": 0,
    "message": "操作成功",
    "data": {
        "RequestId": "E7A855E3-0A3B-479A-BF22-416A2D5D27B1",
        "Credentials": {
            "AccessKeySecret": "<your-access-key-secret>",
            "Expiration": "2021-02-24T07:48:21Z",
            "AccessKeyId": "<your-access-key-id>",
            "SecurityToken": "<your-security-token>"
        },
        "AssumedRoleUser": {
            "AssumedRoleId": "<your-assumed-role-id>",
            "Arn": "acs:ram::<your-account-id>:role/normal-app/normal-app"
        }
    }
}
```

### 获取华为云token
URL: /oss/get-huaweiyun-token

`get`|`post`

**请求参数：**  
无

**返回参数：**  

```json
{
     "result": 0,
     "message": "操作成功",
     "data": {
         "RequestId": "",
         "Credentials": {
             "AccessKeySecret": "<your-access-key-secret>",
             "Expiration": "2021-02-24T07:48:21Z",
             "AccessKeyId": "<your-access-key-id>",
             "SecurityToken": "<your-security-token>"
         },
         "AssumedRoleUser": {
             "AssumedRoleId": "",
             "Arn": ""
         }
     }
 }
```