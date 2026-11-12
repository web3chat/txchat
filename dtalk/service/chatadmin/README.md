# 运营管理后台 (ChatAdmin)

## 项目概述

群组管理服务是 dtalk 项目中的一个独立微服务，专门负责群组数据库相关功能的管理。该服务提供了完整的群组信息管理、白名单管理等功能，通过 HTTP API 对外提供服务。

## 主要功能

### 1. 群组信息管理
- **查询群组信息**: 支持根据群组ID获取详细信息
- **群组列表查询**: 支持分页查询、关键词搜索、状态筛选
- **群组信息修改**: 支持修改群组最大人数、群组状态、群组名称
- **群组统计**: 提供群组总数、活跃群组、封禁群组、解散群组等统计信息

### 2. 白名单管理
- **白名单查询**: 支持根据ID或地址查询白名单信息
- **白名单列表**: 支持分页查询、关键词搜索、状态筛选
- **白名单操作**: 支持添加、更新、删除白名单用户
- **白名单统计**: 提供白名单总数、启用数量、禁用数量等统计信息

## 技术栈

- **编程语言**: Go 1.19+
- **Web框架**: Gin
- **数据库**: MySQL 8.0+
- **缓存**: Redis 6.0+
- **文档**: Swagger
- **构建工具**: Make

## 项目架构

```
chatadmin/
├── cmd/                    # 主程序入口
│   └── main.go
├── config/                 # 配置文件
│   ├── config.go
│   └── groupmanage.toml
├── dao/                    # 数据访问层
│   ├── groupinfo.go
│   └── whitelist.go
├── model/                  # 数据模型
│   ├── biz/               # 业务模型
│   ├── db/                # 数据库模型
│   └── types/             # 类型定义
├── server/                # 服务器
│   └── http/              # HTTP服务器
│       ├── http.go
│       └── handlers.go
├── service/               # 业务逻辑层
│   └── service.go
├── docs/                  # API文档
├── build/                 # 构建输出
├── Makefile              # 构建脚本
└── README.md             # 项目说明
```

## 数据库表结构

### dtalk_group_info 表
群组信息表，包含群组的基本信息和配置。

主要字段：
- `group_id`: 群组ID
- `group_name`: 群组名称
- `group_maximum`: 群组最大人数
- `group_status`: 群组状态 (0=正常, 1=封禁, 2=解散)
- `group_owner_id`: 群主ID
- `group_create_time`: 创建时间
- `group_update_time`: 更新时间

### group_create_whitelist 表
群组创建白名单表，控制哪些用户可以创建群组。

主要字段：
- `id`: 主键ID
- `address`: 用户地址
- `remark`: 备注说明
- `status`: 状态 (1=启用, 0=禁用)
- `max_members`: 群聊最大人数上限
- `created_by`: 创建人地址
- `created_at`: 创建时间
- `updated_at`: 更新时间

## API 接口

### 群组管理接口

#### 获取群组列表
```
GET /api/v1/chatadmin/groups/list
```
参数：
- `page`: 页码 (默认: 1)
- `pageSize`: 每页数量 (默认: 10)
- `keyword`: 搜索关键词
- `status`: 群组状态 (0=正常, 1=封禁, 2=解散)

#### 获取群组信息
```
GET /api/v1/chatadmin/groups/{id}
```

#### 更新群组最大人数
```
PUT /api/v1/chatadmin/groups/{id}/maximum
```

#### 更新群组状态
```
PUT /api/v1/chatadmin/groups/{id}/status
```

#### 更新群组名称
```
PUT /api/v1/chatadmin/groups/{id}/name
```

#### 获取群组统计
```
GET /api/v1/chatadmin/groups/stats
```

### 白名单管理接口

#### 获取白名单列表
```
GET /api/v1/chatadmin/whitelist/list
```
参数：
- `page`: 页码 (默认: 1)
- `pageSize`: 每页数量 (默认: 10)
- `keyword`: 搜索关键词
- `status`: 状态 (1=启用, 0=禁用)

#### 获取白名单信息
```
GET /api/v1/chatadmin/whitelist/{id}
```

#### 添加白名单
```
POST /api/v1/chatadmin/whitelist
```

#### 更新白名单
```
PUT /api/v1/chatadmin/whitelist/{id}
```

#### 删除白名单
```
DELETE /api/v1/chatadmin/whitelist/{id}
```

#### 获取白名单统计
```
GET /api/v1/chatadmin/whitelist/stats
```

## 快速开始

### 1. 环境要求
- Go 1.19+
- MySQL 8.0+
- Redis 6.0+

### 2. 安装依赖
```bash
make deps
```

### 3. 配置数据库
确保 MySQL 数据库中存在以下表：
- `dtalk_group_info`
- `group_create_whitelist`

### 4. 配置服务
修改 `config/chatadmin.toml` 配置文件：
```toml
# 环境配置
Env = "debug"

# HTTP服务器配置
[HttpServer]
Addr = "0.0.0.0:18020"

# MySQL数据库配置
[MySQL]
Host = "127.0.0.1"
Port = 3306
User = "root"
Pwd = "123456"
Db = "dtalk"

# Redis配置
[Redis]
Network = "tcp"
Addr = "127.0.0.1:6379"
```

### 5. 构建和运行
```bash
# 构建项目
make build

# 运行服务
make run

# 或者开发模式运行
make dev
```

### 6. 访问服务
- 服务地址: http://localhost:18020
- API文档: http://localhost:18020/swagger/index.html

## 构建命令

```bash
# 构建二进制文件
make build

# 快速构建（仅编译二进制）
make quick_build

# 构建所有架构
make build_all

# 运行服务
make run

# 开发模式运行
make dev

# 运行测试
make test

# 代码格式化
make fmt

# 代码检查
make lint

# 生成swagger文档
make swag

# 清理构建文件
make clean

# 安装依赖
make deps

# 显示版本信息
make version
```

## 设计模式

### 1. 分层架构
- **Handler层**: 处理HTTP请求和响应
- **Service层**: 业务逻辑处理
- **DAO层**: 数据访问操作
- **Model层**: 数据模型定义

### 2. 依赖注入
通过构造函数注入依赖，便于测试和维护。

### 3. 接口抽象
使用接口定义DAO层，便于扩展和测试。

## 错误处理

服务使用统一的错误响应格式：
```json
{
  "code": 200,
  "message": "success",
  "data": {}
}
```

## 日志记录

服务使用结构化日志记录，支持不同级别的日志输出。

## 性能优化

1. **数据库连接池**: 使用连接池管理数据库连接
2. **分页查询**: 支持大数据量的分页查询
3. **索引优化**: 数据库表添加了必要的索引
4. **缓存支持**: 集成Redis缓存支持

## 监控和运维

- 支持pprof性能分析
- 结构化日志输出
- 健康检查接口
- 优雅关闭支持

## 开发指南

### 添加新的API接口
1. 在 `model/biz/` 中定义请求和响应结构
2. 在 `dao/` 中实现数据访问方法
3. 在 `service/` 中实现业务逻辑
4. 在 `server/http/handlers.go` 中添加HTTP处理器
5. 在 `server/http/http.go` 中注册路由

### 数据库迁移
1. 在 `sql/` 目录下创建迁移脚本
2. 更新数据库表结构
3. 更新对应的模型定义

## 许可证

本项目采用 MIT 许可证。
