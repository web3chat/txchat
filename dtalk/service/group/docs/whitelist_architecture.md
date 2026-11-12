# 群组创建白名单功能架构图

## 系统架构

```mermaid
graph TB
    subgraph "客户端层"
        A[HTTP Client]
        B[gRPC Client]
    end
    
    subgraph "网关层"
        C[Gateway HTTP]
        D[Gateway gRPC]
    end
    
    subgraph "中间件层"
        E[WhitelistMiddleware]
        F[WhitelistInterceptor]
    end
    
    subgraph "业务逻辑层"
        G[CreateGroupLogic]
        H[AddressHelper]
    end
    
    subgraph "数据访问层"
        I[WhitelistDAO]
        J[MySQL Database]
    end
    
    subgraph "数据存储"
        K[dtalk_group_whitelist]
        L[Redis Cache]
    end
    
    A --> C
    B --> D
    C --> E
    D --> F
    E --> G
    F --> G
    G --> H
    G --> I
    I --> J
    J --> K
    I --> L
    
    classDef client fill:#e1f5fe
    classDef gateway fill:#f3e5f5
    classDef middleware fill:#fff3e0
    classDef logic fill:#e8f5e8
    classDef dao fill:#fce4ec
    classDef storage fill:#f1f8e9
    
    class A,B client
    class C,D gateway
    class E,F middleware
    class G,H logic
    class I dao
    class J,K,L storage
```

## 数据流图

```mermaid
sequenceDiagram
    participant Client as 客户端
    participant Gateway as 网关
    participant Middleware as 中间件
    participant Logic as 业务逻辑
    participant DAO as 数据访问
    participant DB as 数据库
    
    Client->>Gateway: HTTP请求 (带地址信息)
    Gateway->>Middleware: 提取地址
    Middleware->>Logic: 传递地址到Context
    Logic->>DAO: 检查白名单权限
    DAO->>DB: 查询白名单表
    
    alt 在白名单中
        DB-->>DAO: 返回白名单信息
        DAO-->>Logic: 返回权限和限制
        Logic->>Logic: 检查人数限制
        Logic->>Logic: 创建群组
        Logic-->>Gateway: 返回成功
        Gateway-->>Client: 返回群组ID
    else 不在白名单中
        DB-->>DAO: 返回空结果
        DAO-->>Logic: 返回无权限
        Logic-->>Gateway: 返回权限错误
        Gateway-->>Client: 返回错误信息
    end
```

## 核心组件说明

### 1. 数据模型层
- **WhitelistInfo**: 白名单信息结构体
- **CheckWhitelistResp**: 白名单检查响应
- **ErrGroupCreatePermission**: 权限错误定义

### 2. 数据访问层
- **WhitelistDAO**: 白名单数据访问接口
- 支持增删改查操作
- 支持按地址查询权限和限制

### 3. 业务逻辑层
- **CreateGroupLogic**: 群组创建逻辑
- **AddressHelper**: 地址获取和验证工具
- 集成白名单检查到创建流程

### 4. 中间件层
- **WhitelistMiddleware**: HTTP层地址提取
- **WhitelistInterceptor**: gRPC层地址提取
- 支持多种地址获取方式

### 5. 数据存储层
- **MySQL**: 主数据存储
- **Redis**: 缓存层（可选）
- 支持高并发查询

## 关键特性

1. **权限控制**: 只有白名单中的地址才能创建群组
2. **人数限制**: 每个地址可以设置最大群成员数量
3. **灵活配置**: 支持动态添加/删除白名单记录
4. **高性能**: 支持缓存和索引优化
5. **可扩展**: 支持多种地址获取方式

## 使用场景

1. **企业内网**: 限制只有特定员工可以创建群组
2. **VIP用户**: 为付费用户提供更大的群组容量
3. **安全控制**: 防止恶意用户创建大量群组
4. **资源管理**: 合理分配群组创建权限
