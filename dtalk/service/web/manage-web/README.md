# 群组管理系统 - Web管理界面

## 项目简介

这是一个基于Web的群组管理系统后台管理界面，用于管理和监控群组信息、白名单等数据。系统采用现代化的前端技术，提供直观、易用的管理界面。

## 功能特性

### 🎯 核心功能
- **仪表盘**: 实时显示系统统计信息和数据概览
- **群组管理**: 查看、编辑、搜索群组信息
- **白名单管理**: 管理群组创建白名单
- **数据导出**: 支持CSV格式数据导出
- **实时更新**: 自动刷新数据，保持信息同步

### 🎨 界面特性
- **响应式设计**: 支持桌面端和移动端访问
- **现代化UI**: 采用渐变色彩和动画效果
- **直观操作**: 简洁明了的用户界面
- **数据可视化**: 图表和统计卡片展示

### 🔧 技术特性
- **纯前端实现**: 无需服务器，直接部署
- **模块化架构**: 清晰的代码组织结构
- **API集成**: 与后端API无缝对接
- **本地存储**: 配置信息本地保存

## 目录结构

```
web/
├── index.html              # 主页面
├── css/                    # 样式文件
│   ├── style.css          # 主样式文件
│   └── dashboard.css      # 仪表盘专用样式
├── js/                     # JavaScript文件
│   ├── api.js             # API调用模块
│   ├── app.js             # 主应用模块
│   ├── groups.js          # 群组管理模块
│   └── whitelist.js       # 白名单管理模块
├── images/                 # 图片资源
├── api/                    # API相关文件
├── config.js              # 配置文件
└── README.md              # 说明文档
```

## 快速开始

### 1. 部署方式

#### 方式一：直接打开
```bash
# 直接在浏览器中打开 index.html 文件
open index.html
```

#### 方式二：本地服务器
```bash
# 使用Python启动本地服务器
python -m http.server 8000

# 或使用Node.js
npx http-server

# 然后访问 http://localhost:8000
```

#### 方式三：Nginx部署
```nginx
server {
    listen 80;
    server_name your-domain.com;
    root /path/to/web;
    index index.html;
    
    location / {
        try_files $uri $uri/ /index.html;
    }
}
```

### 2. 配置API地址

1. 打开系统设置页面
2. 配置API基础URL（默认：http://localhost:18020）
3. 设置请求超时时间（默认：30秒）
4. 点击保存设置

### 3. 开始使用

1. **查看仪表盘**: 了解系统整体状况
2. **管理群组**: 查看、编辑群组信息
3. **管理白名单**: 添加、编辑、删除白名单
4. **导出数据**: 将数据导出为CSV格式

## 功能说明

### 仪表盘
- 显示群组总数、正常群组、封禁群组、解散群组数量
- 显示白名单总数、启用白名单、禁用白名单数量
- 提供快速操作按钮
- 支持数据刷新

### 群组管理
- **列表查看**: 分页显示群组列表
- **搜索功能**: 按群组名称或ID搜索
- **状态筛选**: 按群组状态筛选
- **详情查看**: 查看群组详细信息
- **编辑功能**: 修改群组名称、最大人数、状态
- **状态查询**: 按状态查询群组

### 白名单管理
- **列表查看**: 分页显示白名单列表
- **添加白名单**: 新增白名单用户
- **编辑白名单**: 修改白名单信息
- **删除白名单**: 删除不需要的白名单
- **批量操作**: 批量启用、禁用、删除
- **数据导出**: 导出白名单数据

### 系统设置
- **API配置**: 设置API地址和超时时间
- **连接测试**: 测试API连接状态
- **系统信息**: 显示版本和更新时间

## API接口说明

系统需要后端提供以下API接口：

### 群组管理接口
- `GET /api/v1/groupmanage/groups/list` - 获取群组列表
- `GET /api/v1/groupmanage/groups/stats` - 获取群组统计
- `GET /api/v1/groupmanage/groups/info/{id}` - 获取群组详情
- `PUT /api/v1/groupmanage/groups/maximum/{id}` - 更新群组最大人数
- `PUT /api/v1/groupmanage/groups/status/{id}` - 更新群组状态
- `PUT /api/v1/groupmanage/groups/name/{id}` - 更新群组名称
- `GET /api/v1/groupmanage/groups/by-status/{status}` - 按状态查询群组

### 白名单管理接口
- `GET /api/v1/groupmanage/whitelist/list` - 获取白名单列表
- `GET /api/v1/groupmanage/whitelist/stats` - 获取白名单统计
- `GET /api/v1/groupmanage/whitelist/info/{id}` - 获取白名单详情
- `POST /api/v1/groupmanage/whitelist/` - 添加白名单
- `PUT /api/v1/groupmanage/whitelist/update/{id}` - 更新白名单
- `DELETE /api/v1/groupmanage/whitelist/delete/{id}` - 删除白名单

## 浏览器兼容性

- Chrome 60+
- Firefox 55+
- Safari 12+
- Edge 79+

## 开发说明

### 代码结构
- **api.js**: 封装所有API调用，提供统一的接口
- **app.js**: 主应用逻辑，处理页面导航和全局功能
- **groups.js**: 群组管理相关功能
- **whitelist.js**: 白名单管理相关功能
- **config.js**: 配置文件，存储应用配置

### 自定义开发
1. 修改 `config.js` 调整配置
2. 在对应模块中添加新功能
3. 更新 `index.html` 添加新界面元素
4. 修改 `style.css` 调整样式

### 扩展功能
- 添加新的数据统计图表
- 实现数据导入功能
- 添加用户权限管理
- 集成更多API接口

## 故障排除

### 常见问题

1. **API连接失败**
   - 检查API地址是否正确
   - 确认后端服务是否启动
   - 检查网络连接

2. **数据加载失败**
   - 检查API接口是否正常
   - 查看浏览器控制台错误信息
   - 确认数据格式是否正确

3. **页面显示异常**
   - 清除浏览器缓存
   - 检查CSS文件是否加载
   - 确认JavaScript文件路径

### 调试方法
1. 打开浏览器开发者工具
2. 查看Console面板的错误信息
3. 检查Network面板的请求状态
4. 查看Application面板的本地存储

## 更新日志

### v1.0.0 (2024-01-01)
- 初始版本发布
- 实现基础群组管理功能
- 实现白名单管理功能
- 添加仪表盘统计功能
- 支持数据导出功能

## 许可证

MIT License

## 联系方式

如有问题或建议，请联系开发团队。
