/**
 * 配置文件
 * 存储应用的配置信息
 */

const CONFIG = {
    // API配置
    API: {
        // 空字符串表示与当前页面同源。页面由 chatadmin 挂在 /console/ 时，接口就是这个主机的 18020。
        // 单独把页面开在别的端口时，再写成 http://<chatadmin主机>:18020。
        BASE_URL: '',
        TIMEOUT: 30000,
        RETRY_COUNT: 3,
        RETRY_DELAY: 1000
    },
    
    // 分页配置
    PAGINATION: {
        DEFAULT_PAGE_SIZE: 10,
        PAGE_SIZE_OPTIONS: [10, 20, 50, 100],
        MAX_PAGE_SIZE: 1000
    },
    
    // 界面配置
    UI: {
        THEME: 'light', // light, dark
        LANGUAGE: 'zh-CN',
        ANIMATION_DURATION: 300,
        NOTIFICATION_DURATION: 3000
    },
    
    // 数据配置
    DATA: {
        CACHE_DURATION: 5 * 60 * 1000, // 5分钟
        AUTO_REFRESH_INTERVAL: 30 * 1000, // 30秒
        EXPORT_MAX_RECORDS: 10000
    },
    
    // 安全配置
    SECURITY: {
        SESSION_TIMEOUT: 30 * 60 * 1000, // 30分钟
        MAX_LOGIN_ATTEMPTS: 5,
        LOCKOUT_DURATION: 15 * 60 * 1000 // 15分钟
    },
    
    // 功能开关
    FEATURES: {
        ENABLE_EXPORT: true,
        ENABLE_BATCH_OPERATIONS: true,
        ENABLE_REAL_TIME_UPDATES: true,
        ENABLE_DATA_CACHING: true,
        ENABLE_OFFLINE_MODE: false
    },
    
    // 状态码映射
    STATUS_CODES: {
        SUCCESS: 200,
        CREATED: 201,
        BAD_REQUEST: 400,
        UNAUTHORIZED: 401,
        FORBIDDEN: 403,
        NOT_FOUND: 404,
        INTERNAL_ERROR: 500
    },
    
    // 状态文本映射
    STATUS_TEXT: {
        GROUP: {
            0: '正常',
            1: '封禁',
            2: '解散'
        },
        WHITELIST: {
            0: '禁用',
            1: '启用'
        }
    },
    
    // 状态颜色映射
    STATUS_COLORS: {
        GROUP: {
            0: '#4CAF50', // 绿色 - 正常
            1: '#f44336', // 红色 - 封禁
            2: '#ff9800'  // 橙色 - 解散
        },
        WHITELIST: {
            0: '#9e9e9e', // 灰色 - 禁用
            1: '#4CAF50'  // 绿色 - 启用
        }
    },
    
    // 表格列配置
    TABLE_COLUMNS: {
        GROUPS: [
            { key: 'groupId', label: '群组ID', width: '100px', sortable: true },
            { key: 'groupName', label: '群组名称', width: '200px', sortable: true },
            { key: 'groupMemberNum', label: '成员数', width: '100px', sortable: true, type: 'number' },
            { key: 'groupMaximum', label: '最大人数', width: '100px', sortable: true, type: 'number' },
            { key: 'groupStatus', label: '状态', width: '100px', sortable: true, type: 'status' },
            { key: 'groupCreateTime', label: '创建时间', width: '150px', sortable: true, type: 'datetime' },
            { key: 'actions', label: '操作', width: '150px', sortable: false }
        ],
        WHITELIST: [
            { key: 'id', label: 'ID', width: '80px', sortable: true },
            { key: 'address', label: '用户地址', width: '200px', sortable: true },
            { key: 'remark', label: '备注', width: '150px', sortable: true },
            { key: 'maxMembers', label: '最大成员数', width: '120px', sortable: true, type: 'number' },
            { key: 'status', label: '状态', width: '100px', sortable: true, type: 'status' },
            { key: 'createdBy', label: '授权人', width: '120px', sortable: true },
            { key: 'createdAt', label: '创建时间', width: '150px', sortable: true, type: 'datetime' },
            { key: 'actions', label: '操作', width: '150px', sortable: false }
        ]
    },
    
    // 表单验证规则
    VALIDATION_RULES: {
        GROUP_NAME: {
            required: true,
            minLength: 1,
            maxLength: 50,
            pattern: /^[\u4e00-\u9fa5a-zA-Z0-9\s]+$/
        },
        GROUP_MAXIMUM: {
            required: true,
            min: 1,
            max: 10000,
            type: 'number'
        },
        WHITELIST_ADDRESS: {
            required: true,
            minLength: 1,
            maxLength: 100,
            pattern: /^[a-zA-Z0-9_-]+$/
        },
        WHITELIST_MAX_MEMBERS: {
            required: true,
            min: 1,
            max: 10000,
            type: 'number'
        },
        CREATED_BY: {
            required: true,
            minLength: 1,
            maxLength: 50,
            pattern: /^[a-zA-Z0-9_-]+$/
        }
    },
    
    // 导出配置
    EXPORT: {
        CSV_ENCODING: 'utf-8',
        CSV_DELIMITER: ',',
        DATE_FORMAT: 'YYYY-MM-DD HH:mm:ss',
        NUMBER_FORMAT: '0,0'
    },
    
    // 错误消息
    ERROR_MESSAGES: {
        NETWORK_ERROR: '网络连接失败，请检查网络设置',
        API_ERROR: 'API请求失败，请稍后重试',
        VALIDATION_ERROR: '输入数据格式不正确',
        PERMISSION_ERROR: '没有权限执行此操作',
        NOT_FOUND_ERROR: '请求的资源不存在',
        SERVER_ERROR: '服务器内部错误，请联系管理员'
    },
    
    // 成功消息
    SUCCESS_MESSAGES: {
        SAVE_SUCCESS: '保存成功',
        DELETE_SUCCESS: '删除成功',
        UPDATE_SUCCESS: '更新成功',
        EXPORT_SUCCESS: '导出成功',
        IMPORT_SUCCESS: '导入成功'
    },
    
    // 警告消息
    WARNING_MESSAGES: {
        UNSAVED_CHANGES: '有未保存的更改，确定要离开吗？',
        DELETE_CONFIRM: '确定要删除吗？此操作不可撤销',
        BATCH_DELETE_CONFIRM: '确定要删除选中的项目吗？此操作不可撤销',
        EXPORT_LARGE_DATA: '数据量较大，导出可能需要较长时间'
    }
};

// 导出配置
window.CONFIG = CONFIG;
