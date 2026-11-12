/**
 * API调用模块
 * 封装所有与后端API的交互
 */

function defaultApiBaseURLFromConfig() {
    if (typeof CONFIG !== 'undefined' && CONFIG.API && CONFIG.API.BASE_URL) {
        return String(CONFIG.API.BASE_URL).replace(/\/$/, '');
    }
    if (typeof window !== 'undefined' && window.location && window.location.origin) {
        return window.location.origin;
    }
    return 'http://127.0.0.1:18020';
}

function appendQueryValues(query, key, value) {
    const list = Array.isArray(value) ? value : (value ? [value] : []);
    list.forEach((item) => {
        const text = String(item || '').trim();
        if (text) query.append(key, text);
    });
}

class APIClient {
    constructor() {
        this.baseURL =
            localStorage.getItem('apiBaseURL') || defaultApiBaseURLFromConfig();
        this.timeout = parseInt(localStorage.getItem('apiTimeout')) || 30000;
        this.apiBase = '/api/v1/groupmanage';
    }

    /**
     * 更新API配置
     */
    updateConfig(baseURL, timeout) {
        this.baseURL = baseURL;
        this.timeout = timeout;
        localStorage.setItem('apiBaseURL', baseURL);
        localStorage.setItem('apiTimeout', timeout.toString());
    }

    /**
     * 通用请求方法
     */
    async request(endpoint, options = {}) {
        const url = `${this.baseURL}${this.apiBase}${endpoint}`;
        const isAuthPath = endpoint.startsWith('/auth/');
        const token = localStorage.getItem('accessToken');
        // 未登录时阻断除auth外的所有请求，避免产生401噪音
        if (!isAuthPath && (!token || token === 'null' || token === 'undefined')) {
            throw new Error('未登录或会话已失效');
        }
        const defaultOptions = {
            method: 'GET',
            timeout: this.timeout,
        };

        // 深合并headers，避免调用处覆盖掉Authorization
        const baseHeaders = { 'Content-Type': 'application/json' };
        const mergedHeaders = { ...baseHeaders, ...(options.headers || {}) };
        if (token) {
            mergedHeaders['Authorization'] = `Bearer ${token}`;
        }

        // 可临时打开调试
        try { console.debug && console.debug('Auth header', mergedHeaders['Authorization']); } catch (e) {}

        const config = { ...defaultOptions, ...options, headers: mergedHeaders };

        try {
            showLoading();
            const response = await fetch(url, config);
            
            // 先读取响应文本（body只能读取一次）
            const responseText = await response.text();
            
            // 获取Content-Type
            const contentType = response.headers.get('content-type') || '';
            const isJson = contentType.includes('application/json');
            
            // 401 统一处理：清理登录态并提示
            if (response.status === 401) {
                let errorMessage = '未授权，请重新登录';
                if (isJson && responseText) {
                    try {
                        const data = JSON.parse(responseText);
                        errorMessage = data.message || errorMessage;
                    } catch(e) {
                        // JSON解析失败，使用默认消息
                        console.warn('401响应JSON解析失败:', e);
                    }
                }
                showNotification(errorMessage, 'warning');
                if (typeof logoutAdmin === 'function') logoutAdmin();
                throw new Error('未授权');
            }
            
            // 检查响应是否为JSON格式
            if (!isJson) {
                console.error('非JSON响应:', {
                    status: response.status,
                    statusText: response.statusText,
                    contentType: contentType,
                    preview: responseText.substring(0, 200),
                    url: url
                });
                
                if (!response.ok) {
                    throw new Error(`HTTP ${response.status}: ${response.statusText}`);
                } else {
                    throw new Error('服务器返回了非JSON格式的响应');
                }
            }
            
            // 解析JSON响应
            let data;
            try {
                if (!responseText || responseText.trim() === '') {
                    throw new Error('响应体为空');
                }
                data = JSON.parse(responseText);
            } catch (parseError) {
                console.error('JSON解析失败:', {
                    error: parseError,
                    url: url,
                    status: response.status,
                    statusText: response.statusText,
                    contentType: contentType,
                    responsePreview: responseText.substring(0, 500)
                });
                throw new Error(`JSON解析失败: ${parseError.message}`);
            }

            if (!response.ok) {
                throw new Error(data.message || `HTTP ${response.status}: ${response.statusText}`);
            }

            return data;
        } catch (error) {
            console.error('API请求失败:', {
                error: error,
                url: url,
                method: config.method || 'GET'
            });
            
            // 如果是JSON解析错误，提供更友好的错误信息
            if (error.message && (error.message.includes('JSON') || error.message.includes('Unexpected'))) {
                showNotification('服务器响应格式错误，请检查API地址和网络连接', 'error');
            } else {
                showNotification('请求失败: ' + error.message, 'error');
            }
            throw error;
        } finally {
            hideLoading();
        }
    }

    /**
     * 群组管理API
     */
    async getGroupList(params = {}) {
        const queryString = new URLSearchParams(params).toString();
        const endpoint = `/groups/list${queryString ? '?' + queryString : ''}`;
        return await this.request(endpoint);
    }

    async getGroupStats() {
        return await this.request('/groups/stats');
    }

    async getTodayGroupCount() {
        return await this.request('/groups/today-count');
    }

    async getTotalUserCount() {
        return await this.request('/users/total-count');
    }

    async getTodayNewUserCount() {
        return await this.request('/users/today-new-count');
    }

    async getUserLoginHistory(params = {}) {
        const queryParams = new URLSearchParams();
        if (params.address) queryParams.append('address', params.address);
        const page = Number(params.page);
        const pageNum = Number.isFinite(page) && page > 0 ? Math.floor(page) : 1;
        const ps = Number(params.pageSize);
        const pageSizeNum =
            Number.isFinite(ps) && ps > 0 ? Math.min(100, Math.floor(ps)) : 20;
        queryParams.set('page', String(pageNum));
        queryParams.set('pageSize', String(pageSizeNum));
        if (params.startTime) queryParams.append('startTime', params.startTime);
        if (params.endTime) queryParams.append('endTime', params.endTime);
        const query = queryParams.toString();
        return await this.request(`/users/login-history${query ? '?' + query : ''}`);
    }

    /**
     * 全站登录日志分页（可选多个 uid：并行查询 ?uid=a&uid=b）
     * @param {object} params
     * @param {number} [params.page]
     * @param {number} [params.pageSize]
     * @param {number} [params.startTime]
     * @param {number} [params.endTime]
     * @param {string|string[]} [params.uid] 单个地址或地址数组
     */
    async getAllLoginLogs(params = {}) {
        const queryParams = new URLSearchParams();
        const page = Number(params.page);
        const pageNum = Number.isFinite(page) && page > 0 ? Math.floor(page) : 1;
        const ps = Number(params.pageSize);
        const pageSizeNum =
            Number.isFinite(ps) && ps > 0 ? Math.min(100, Math.floor(ps)) : 20;
        queryParams.set('page', String(pageNum));
        queryParams.set('pageSize', String(pageSizeNum));
        if (params.startTime) queryParams.append('startTime', params.startTime);
        if (params.endTime) queryParams.append('endTime', params.endTime);
        if (params.uid) {
            const arr = Array.isArray(params.uid) ? params.uid : [params.uid];
            arr.forEach((u) => {
                const s = String(u || '').trim();
                if (s) queryParams.append('uid', s);
            });
        }
        const query = queryParams.toString();
        return await this.request(`/users/login-logs?${query}`);
    }

    async logout() {
        return await this.request('/auth/logout', { method: 'POST' });
    }

    async getAudits(params = {}) {
        const queryParams = new URLSearchParams();
        const page = Number(params.page);
        const pageNum = Number.isFinite(page) && page > 0 ? Math.floor(page) : 1;
        const ps = Number(params.pageSize);
        const pageSizeNum = Number.isFinite(ps) && ps > 0 ? Math.min(100, Math.floor(ps)) : 20;
        queryParams.set('page', String(pageNum));
        queryParams.set('pageSize', String(pageSizeNum));
        appendQueryValues(queryParams, 'action', params.action);
        appendQueryValues(queryParams, 'targetType', params.targetType);
        if (params.targetId) queryParams.set('targetId', String(params.targetId).trim());
        if (params.operator) queryParams.set('operator', String(params.operator).trim());
        return await this.request(`/audits?${queryParams.toString()}`);
    }

    async getGroupInfo(groupId) {
        return await this.request(`/groups/info/${groupId}`);
    }

    async updateGroupMaximum(groupId, maximum) {
        return await this.request(`/groups/maximum/${groupId}`, {
            method: 'PUT',
            body: JSON.stringify({ groupMaximum: maximum })
        });
    }

    async updateGroupStatus(groupId, status) {
        return await this.request(`/groups/status/${groupId}`, {
            method: 'PUT',
            body: JSON.stringify({ groupStatus: status })
        });
    }

    async updateGroupName(groupId, name) {
        return await this.request(`/groups/name/${groupId}`, {
            method: 'PUT',
            body: JSON.stringify({ groupName: name })
        });
    }

    async getGroupsByStatus(status, params = {}) {
        const queryString = new URLSearchParams(params).toString();
        const endpoint = `/groups/by-status/${status}${queryString ? '?' + queryString : ''}`;
        return await this.request(endpoint);
    }

    async getGroupMembers(groupId, params = {}) {
        const query = new URLSearchParams();
        if (params.page) query.set('page', String(params.page));
        if (params.pageSize) query.set('pageSize', String(params.pageSize));
        const qs = query.toString();
        return await this.request(`/groups/${groupId}/members${qs ? '?' + qs : ''}`);
    }

    async getGroupsByOwner(ownerId, params = {}) {
        const queryString = new URLSearchParams({ ownerId, ...params }).toString();
        const endpoint = `/groups/by-owner${queryString ? '?' + queryString : ''}`;
        return await this.request(endpoint);
    }

    /**
     * 白名单管理API
     */
    async getWhitelistList(params = {}) {
        const queryString = new URLSearchParams(params).toString();
        const endpoint = `/whitelist/list${queryString ? '?' + queryString : ''}`;
        return await this.request(endpoint);
    }

    async getWhitelistByAddress(address) {
        return await this.request(`/whitelist/by-address?address=${encodeURIComponent(address)}`);
    }

    async getWhitelistStats() {
        return await this.request('/whitelist/stats');
    }

    async getWhitelistInfo(whitelistId) {
        return await this.request(`/whitelist/info/${whitelistId}`);
    }

    /**
     * 获取服务配置信息
     */
    async getConfig() {
        return await this.request('/config', {
            method: 'GET'
        });
    }

    async getDailyActiveList(params = {}) {
        const queryParams = new URLSearchParams();
        if (params.startDate) queryParams.append('startDate', params.startDate);
        if (params.endDate) queryParams.append('endDate', params.endDate);
        if (Array.isArray(params.dates)) {
            params.dates.forEach((d) => {
                if (d) queryParams.append('date', d);
            });
        }
        if (params.page) queryParams.append('page', params.page);
        if (params.pageSize) queryParams.append('pageSize', params.pageSize);
        const query = queryParams.toString();
        return await this.request(`/metrics/daily-active${query ? '?' + query : ''}`);
    }

    async getDailyActiveAddresses(params = {}) {
        const queryParams = new URLSearchParams();
        if (params.date) queryParams.set('date', params.date);
        queryParams.set('page', String(params.page || 1));
        queryParams.set('pageSize', String(params.pageSize || 50));
        return await this.request(`/metrics/daily-active/addresses?${queryParams.toString()}`);
    }

    async getDailyActiveGroups(params = {}) {
        const queryParams = new URLSearchParams();
        if (params.date) queryParams.set('date', params.date);
        queryParams.set('page', String(params.page || 1));
        queryParams.set('pageSize', String(params.pageSize || 50));
        return await this.request(`/metrics/daily-active/groups?${queryParams.toString()}`);
    }

    async snapshotDailyActive(date) {
        const q = date ? `?date=${encodeURIComponent(date)}` : '';
        return await this.request(`/metrics/daily-active/snapshot${q}`, {
            method: 'POST'
        });
    }

    async addWhitelist(data) {
        return await this.request('/whitelist/', {
            method: 'POST',
            body: JSON.stringify(data)
        });
    }

    async updateWhitelist(whitelistId, data) {
        return await this.request(`/whitelist/update/${whitelistId}`, {
            method: 'PUT',
            body: JSON.stringify(data)
        });
    }

    async deleteWhitelist(whitelistId) {
        return await this.request(`/whitelist/delete/${whitelistId}`, {
            method: 'DELETE'
        });
    }
}

// 创建全局API客户端实例
const apiClient = new APIClient();

/**
 * 工具函数
 */

/**
 * 显示加载提示
 */
function showLoading() {
    const loading = document.getElementById('loading');
    if (loading) {
        loading.style.display = 'block';
    }
}

/**
 * 隐藏加载提示
 */
function hideLoading() {
    const loading = document.getElementById('loading');
    if (loading) {
        loading.style.display = 'none';
    }
}

/**
 * 显示通知
 */
function showNotification(message, type = 'success') {
    console.log('showNotification 被调用:', { message, type });
    
    const notification = document.getElementById('notification');
    if (!notification) {
        console.error('通知元素不存在，无法显示消息:', message);
        // 如果通知元素不存在，使用 alert 作为后备方案
        alert(`${type === 'error' ? '错误' : type === 'warning' ? '警告' : '提示'}: ${message}`);
        return;
    }

    const content = notification.querySelector('.notification-content');
    if (!content) {
        console.error('通知内容元素不存在');
        alert(`${type === 'error' ? '错误' : type === 'warning' ? '警告' : '提示'}: ${message}`);
        return;
    }

    const icon = content.querySelector('i');
    const messageEl = content.querySelector('#notification-message');

    if (!icon || !messageEl) {
        console.error('通知图标或消息元素不存在', { icon: !!icon, messageEl: !!messageEl });
        alert(`${type === 'error' ? '错误' : type === 'warning' ? '警告' : '提示'}: ${message}`);
        return;
    }

    // 设置图标和颜色
    if (type === 'success') {
        icon.className = 'fas fa-check-circle';
        notification.style.background = 'linear-gradient(135deg, #4CAF50 0%, #45a049 100%)';
    } else if (type === 'error') {
        icon.className = 'fas fa-exclamation-circle';
        notification.style.background = 'linear-gradient(135deg, #f44336 0%, #d32f2f 100%)';
    } else if (type === 'warning') {
        icon.className = 'fas fa-exclamation-triangle';
        notification.style.background = 'linear-gradient(135deg, #ff9800 0%, #f57c00 100%)';
    } else if (type === 'info') {
        icon.className = 'fas fa-info-circle';
        notification.style.background = 'linear-gradient(135deg, #2196F3 0%, #1976D2 100%)';
    }

    messageEl.textContent = message;
    
    // 确保通知元素可见并设置最高优先级
    notification.style.display = 'block';
    notification.style.zIndex = '99999'; // 确保在所有元素上方
    notification.style.position = 'fixed';
    notification.style.top = '90px';
    notification.style.right = '20px';
    
    // 先移除 show 类，然后添加，确保动画重新触发
    notification.classList.remove('show');
    
    // 强制重排，确保样式生效
    notification.offsetHeight;
    
    // 使用 setTimeout 确保动画能触发
    setTimeout(() => {
        notification.classList.add('show');
        console.log('通知已显示:', { message, type, zIndex: notification.style.zIndex });
    }, 10);

    // 3秒后自动隐藏
    setTimeout(() => {
        notification.classList.remove('show');
        // 动画结束后隐藏
        setTimeout(() => {
            notification.style.display = 'none';
        }, 300);
    }, 3000);
}

/**
 * 格式化时间戳
 */
function formatTimestamp(timestamp) {
    if (!timestamp) return '-';
    const date = new Date(timestamp);
    return date.toLocaleString('zh-CN', {
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit'
    });
}

/**
 * 格式化数字
 */
function formatNumber(num) {
    if (num === null || num === undefined) return '-';
    return num.toLocaleString('zh-CN');
}

/**
 * 格式化大整数（解决JavaScript精度问题）
 */
function formatLargeNumber(num) {
    if (num === null || num === undefined) return '-';
    
    // 如果数字在JavaScript安全范围内，使用标准格式化
    if (num <= Number.MAX_SAFE_INTEGER) {
        return num.toLocaleString('zh-CN');
    }
    
    // 对于超出安全范围的大整数，直接返回字符串
    return num.toString();
}

/**
 * 获取状态标签HTML
 */
function getStatusBadge(status, type = 'group') {
    const statusMap = {
        group: {
            0: { text: '正常', class: 'status-active' },
            1: { text: '封禁', class: 'status-banned' },
            2: { text: '解散', class: 'status-dissolved' }
        },
        whitelist: {
            0: { text: '禁用', class: 'status-disabled' },
            1: { text: '启用', class: 'status-active' }
        }
    };

    const statusInfo = statusMap[type][status] || { text: '未知', class: 'status-disabled' };
    return `<span class="status-badge ${statusInfo.class}">${statusInfo.text}</span>`;
}

/**
 * 创建分页控件
 */
function createPagination(currentPage, totalPages, onPageChange) {
    if (totalPages <= 1) return '';

    let html = '<div class="pagination">';
    
    // 上一页按钮
    html += `<button ${currentPage <= 1 ? 'disabled' : ''} onclick="${onPageChange}(${currentPage - 1})">
        <i class="fas fa-chevron-left"></i> 上一页
    </button>`;

    // 页码按钮
    const startPage = Math.max(1, currentPage - 2);
    const endPage = Math.min(totalPages, currentPage + 2);

    if (startPage > 1) {
        html += `<button onclick="${onPageChange}(1)">1</button>`;
        if (startPage > 2) {
            html += '<span>...</span>';
        }
    }

    for (let i = startPage; i <= endPage; i++) {
        const activeClass = i === currentPage ? 'active' : '';
        html += `<button class="${activeClass}" onclick="${onPageChange}(${i})">${i}</button>`;
    }

    if (endPage < totalPages) {
        if (endPage < totalPages - 1) {
            html += '<span>...</span>';
        }
        html += `<button onclick="${onPageChange}(${totalPages})">${totalPages}</button>`;
    }

    // 下一页按钮
    html += `<button ${currentPage >= totalPages ? 'disabled' : ''} onclick="${onPageChange}(${currentPage + 1})">
        下一页 <i class="fas fa-chevron-right"></i>
    </button>`;

    html += '</div>';
    return html;
}

/**
 * 防抖函数
 */
function debounce(func, wait) {
    let timeout;
    return function executedFunction(...args) {
        const later = () => {
            clearTimeout(timeout);
            func(...args);
        };
        clearTimeout(timeout);
        timeout = setTimeout(later, wait);
    };
}

/**
 * 节流函数
 */
function throttle(func, limit) {
    let inThrottle;
    return function() {
        const args = arguments;
        const context = this;
        if (!inThrottle) {
            func.apply(context, args);
            inThrottle = true;
            setTimeout(() => inThrottle = false, limit);
        }
    };
}

/**
 * 验证表单数据
 */
function validateForm(formData, rules) {
    const errors = {};
    
    for (const field in rules) {
        const rule = rules[field];
        const value = formData[field];
        
        if (rule.required && (!value || value.toString().trim() === '')) {
            errors[field] = `${rule.label || field}是必填项`;
            continue;
        }
        
        if (value && rule.min && value < rule.min) {
            errors[field] = `${rule.label || field}不能小于${rule.min}`;
            continue;
        }
        
        if (value && rule.max && value > rule.max) {
            errors[field] = `${rule.label || field}不能大于${rule.max}`;
            continue;
        }
        
        if (value && rule.pattern && !rule.pattern.test(value)) {
            errors[field] = `${rule.label || field}格式不正确`;
            continue;
        }
    }
    
    return {
        isValid: Object.keys(errors).length === 0,
        errors
    };
}

/**
 * 显示表单错误
 */
function showFormErrors(errors) {
    // 清除之前的错误
    document.querySelectorAll('.form-error').forEach(el => el.remove());
    
    // 显示新的错误
    for (const field in errors) {
        const input = document.getElementById(field);
        if (input) {
            const errorDiv = document.createElement('div');
            errorDiv.className = 'form-error';
            errorDiv.style.color = '#f44336';
            errorDiv.style.fontSize = '0.8rem';
            errorDiv.style.marginTop = '5px';
            errorDiv.textContent = errors[field];
            input.parentNode.appendChild(errorDiv);
        }
    }
}

/**
 * 清除表单错误
 */
function clearFormErrors() {
    document.querySelectorAll('.form-error').forEach(el => el.remove());
}

/**
 * 导出数据为CSV
 */
function exportToCSV(data, filename) {
    if (!data || data.length === 0) {
        showNotification('没有数据可导出', 'warning');
        return;
    }
    
    const headers = Object.keys(data[0]);
    const csvContent = [
        headers.join(','),
        ...data.map(row => headers.map(header => `"${row[header] || ''}"`).join(','))
    ].join('\n');
    
    const blob = new Blob(['\ufeff' + csvContent], { type: 'text/csv;charset=utf-8;' });
    const link = document.createElement('a');
    const url = URL.createObjectURL(blob);
    link.setAttribute('href', url);
    link.setAttribute('download', `${filename}_${new Date().toISOString().split('T')[0]}.csv`);
    link.style.visibility = 'hidden';
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    
    showNotification('数据导出成功', 'success');
}

/**
 * 获取URL参数
 */
function getUrlParams() {
    const params = new URLSearchParams(window.location.search);
    const result = {};
    for (const [key, value] of params) {
        result[key] = value;
    }
    return result;
}

/**
 * 设置URL参数
 */
function setUrlParams(params) {
    const url = new URL(window.location);
    for (const key in params) {
        if (params[key] !== null && params[key] !== undefined && params[key] !== '') {
            url.searchParams.set(key, params[key]);
        } else {
            url.searchParams.delete(key);
        }
    }
    window.history.replaceState({}, '', url);
}

/**
 * 本地存储工具
 */
const storage = {
    set(key, value) {
        try {
            localStorage.setItem(key, JSON.stringify(value));
        } catch (e) {
            console.error('存储失败:', e);
        }
    },
    
    get(key, defaultValue = null) {
        try {
            const item = localStorage.getItem(key);
            return item ? JSON.parse(item) : defaultValue;
        } catch (e) {
            console.error('读取失败:', e);
            return defaultValue;
        }
    },
    
    remove(key) {
        try {
            localStorage.removeItem(key);
        } catch (e) {
            console.error('删除失败:', e);
        }
    }
};

// 导出API客户端供其他模块使用
window.apiClient = apiClient;
