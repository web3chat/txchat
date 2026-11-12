/**
 * 主应用模块
 * 处理页面导航、数据加载和全局功能
 */

// 全局状态
const appState = {
    currentSection: 'dashboard',
    currentPage: 1,
    pageSize: 10,
    searchKeyword: '',
    statusFilter: '',
    lastUpdate: null,
    adminLevel: -1 // 管理员级别: 0=普通管理员, 1=超级管理员, -1=未找到或已禁用
};

const PAGE_META = {
    dashboard: { title: '工作台', desc: '群组、用户与日活一屏掌握' },
    activity: { title: '活跃趋势', desc: '日活用户与日活群组历史趋势' },
    groups: { title: '群组管理', desc: '查询与调整群组状态、容量' },
    userlogin: { title: '用户查询', desc: '绑定信息与登录历史' },
    loginlogs: { title: '登录日志', desc: '全站登录审计流水' },
    audits: { title: '操作留痕', desc: '管理员登录、修改与删除记录' },
    settings: { title: '系统设置', desc: 'API 与本地偏好' }
};

function toggleSidebar(force) {
    const open = typeof force === 'boolean' ? force : !document.body.classList.contains('sidebar-open');
    document.body.classList.toggle('sidebar-open', open);
}

function updatePageChrome(sectionName) {
    const meta = PAGE_META[sectionName] || { title: sectionName, desc: '' };
    const titleEl = document.getElementById('ca-page-title');
    const descEl = document.getElementById('ca-page-desc');
    if (titleEl) titleEl.textContent = meta.title;
    if (descEl) descEl.textContent = meta.desc;
    if (window.innerWidth <= 960) {
        toggleSidebar(false);
    }
}

window.toggleSidebar = toggleSidebar;

// 页面加载完成后初始化
document.addEventListener('DOMContentLoaded', async function() {
    // 先检查登录状态并切换页面显示
    if (window.toggleLoginOverlay) {
        window.toggleLoginOverlay();
    }
    
    // 如果已登录，先加载配置再初始化应用
    if (window.isLoggedIn && window.isLoggedIn()) {
        // 先加载服务配置信息（需要鉴权），等待完成后再初始化
        await loadServiceConfig();
        initializeApp();
        if (typeof loadDashboardData === 'function') {
            loadDashboardData();
        }
    } else {
        // 未登录时，确保业务页面完全隐藏
        const appContainer = document.getElementById('app-container');
        if (appContainer) {
            appContainer.style.display = 'none';
        }
    }
    setupEventListeners();
    applyRefreshMode();
});

/**
 * 加载服务配置信息
 */
async function loadServiceConfig() {
    try {
        const response = await apiClient.getConfig();
        console.log('配置加载响应:', response);
        if (response && response.data) {
            appState.adminLevel = response.data.adminLevel !== undefined ? response.data.adminLevel : -1;
            console.log('管理员级别:', appState.adminLevel);
        } else {
            console.warn('配置响应数据格式不正确:', response);
            appState.adminLevel = -1;
        }
    } catch (error) {
        console.error('加载服务配置失败:', error);
        appState.adminLevel = -1;
    }
}

/**
 * 初始化应用
 */
function initializeApp() {
    // 从本地存储恢复设置
    const savedSettings = storage.get('appSettings', {});
    if (savedSettings.apiBaseURL) {
        apiClient.updateConfig(savedSettings.apiBaseURL, savedSettings.apiTimeout || 30000);
    }
    
    // 设置导航
    setupNavigation();
    
    // 初始化设置页面
    initializeSettings();
    
    // 显示当前时间
    updateLastUpdateTime();
}

/**
 * 设置导航功能
 */
function setupNavigation() {
    // 确保只在登录状态下设置导航
    if (!isLoggedIn()) {
        return;
    }
    
    const navLinks = document.querySelectorAll('.nav-link');
    navLinks.forEach(link => {
        link.addEventListener('click', function(e) {
            e.preventDefault();
            const section = this.getAttribute('data-section');
            showSection(section);
        });
    });
}

/**
 * 显示指定页面
 */
function showSection(sectionName) {
    // 检查登录状态
    if (!isLoggedIn()) {
        showNotification('请先登录', 'warning');
        return;
    }

    if (sectionName === 'whitelist') {
        return;
    }
    
    // 隐藏所有页面
    const sections = document.querySelectorAll('.content-section');
    sections.forEach(section => {
        section.classList.remove('active');
    });
    
    // 显示目标页面
    const targetSection = document.getElementById(sectionName);
    if (targetSection) {
        targetSection.classList.add('active');
        appState.currentSection = sectionName;
        updatePageChrome(sectionName);
        document.body.classList.remove('whitelist-active');
        const mainContent = document.querySelector('.main-content');
        if (mainContent) {
            mainContent.classList.remove('whitelist-expanded');
        }

        // 更新导航状态
        updateNavigationState(sectionName);
        
        // 根据页面加载相应数据
        switch(sectionName) {
            case 'dashboard':
                if (typeof loadDashboardData === 'function') {
                    loadDashboardData();
                }
                break;
            case 'activity':
                if (typeof loadDailyActiveHistory === 'function') {
                    loadDailyActiveHistory();
                }
                break;
            case 'groups':
                if (typeof loadGroupsData === 'function') {
                    loadGroupsData();
                }
                break;
            case 'settings':
                if (typeof loadSettingsData === 'function') {
                    loadSettingsData();
                }
                break;
            case 'loginlogs':
                if (typeof loginLogsAllModule !== 'undefined' && loginLogsAllModule.onSectionShown) {
                    loginLogsAllModule.onSectionShown();
                }
                break;
            case 'audits':
                if (typeof auditsModule !== 'undefined' && auditsModule.onSectionShown) {
                    auditsModule.onSectionShown();
                }
                break;
        }
    }
}

/**
 * 更新导航状态
 */
function updateNavigationState(activeSection) {
    // 确保只在登录状态下更新导航
    if (!isLoggedIn()) {
        return;
    }
    
    const navLinks = document.querySelectorAll('.nav-link');
    navLinks.forEach(link => {
        link.classList.remove('active');
        if (link.getAttribute('data-section') === activeSection) {
            link.classList.add('active');
        }
    });
}

/**
 * 设置事件监听器
 */
function setupEventListeners() {
    // 搜索功能
    const searchInput = document.getElementById('group-search');
    if (searchInput) {
        searchInput.addEventListener('input', debounce(function() {
            appState.searchKeyword = this.value;
            if (appState.currentSection === 'groups') {
                loadGroupsData();
            }
        }, 500));
    }
    
    // 状态筛选
    const statusFilter = document.getElementById('group-status-filter');
    if (statusFilter) {
        statusFilter.addEventListener('change', function() {
            appState.statusFilter = this.value;
            if (appState.currentSection === 'groups') {
                loadGroupsData();
            }
        });
    }
    
    // 模态框关闭
    document.addEventListener('click', function(e) {
        if (e.target.classList.contains('modal')) {
            e.target.style.display = 'none';
        }
    });
    
    // ESC键关闭模态框
    document.addEventListener('keydown', function(e) {
        if (e.key === 'Escape') {
            const modals = document.querySelectorAll('.modal');
            modals.forEach(modal => {
                if (modal.style.display === 'block') {
                    modal.style.display = 'none';
                }
            });
        }
    });
}

/**
 * 加载仪表盘数据
 */
async function loadDashboardData() {
    try {
        if (window.requireLogin && !window.requireLogin()) {
            maskDashboard();
            return;
        }
        const [groupStats, todayGroupCount, totalUserCount, todayNewUserCount] = await Promise.all([
            apiClient.getGroupStats(),
            apiClient.getTodayGroupCount(),
            apiClient.getTotalUserCount(),
            apiClient.getTodayNewUserCount(),
        ]);
        const userTotal = totalUserCount.data ? totalUserCount.data.count : 0;
        updateGroupStats(groupStats.data, userTotal);
        updateTodayGroupCount(todayGroupCount.data.count);
        updateTotalUserCount(userTotal);
        updateTodayNewUserCount(todayNewUserCount.data.count);

        if (typeof loadDashboardDailyActive === 'function') {
            await loadDashboardDailyActive();
        }
        
        appState.lastUpdate = new Date();
        updateLastUpdateTime();
        
    } catch (error) {
        console.error('加载仪表盘数据失败:', error);
        showNotification('加载数据失败', 'error');
        const dashSync = document.getElementById('dash-last-update');
        if (dashSync) {
            dashSync.innerHTML = '<i class="fas fa-exclamation-circle"></i> 加载失败，请刷新重试';
        }
    }
}

// 未登录时的遮罩：把关键数值清空并提示
function maskDashboard() {
    const ids = ['total-groups','active-groups','banned-groups','dissolved-groups','total-whitelist','active-whitelist','disabled-whitelist','today-groups','total-users','today-new-users','today-active-users','today-active-groups','today-active-users-ratio','today-active-groups-ratio'];
    ids.forEach(id => {
        const el = document.getElementById(id);
        if (el) el.textContent = '-';
    });
}

/**
 * 更新群组统计显示
 */
function updateDashboardStatusRing(stats) {
    const ring = document.getElementById('dash-status-ring');
    if (!ring) {
        return;
    }
    const a = Number(stats.activeGroups) || 0;
    const b = Number(stats.bannedGroups) || 0;
    const c = Number(stats.dissolvedGroups) || 0;
    const t = a + b + c;
    if (t === 0) {
        ring.style.background = 'conic-gradient(#d4d9e0 0deg, #d4d9e0 360deg)';
        return;
    }
    const degA = (a / t) * 360;
    const degB = (b / t) * 360;
    const e1 = degA;
    const e2 = degA + degB;
        ring.style.background =
        'conic-gradient(#059669 0deg ' + e1 + 'deg, #e11d48 ' + e1 + 'deg ' + e2 + 'deg, #94a3b8 ' + e2 + 'deg 360deg)';
}

function updateGroupStats(stats, totalUsers) {
    const elements = {
        'total-groups': stats.totalGroups,
        'active-groups': stats.activeGroups,
        'banned-groups': stats.bannedGroups,
        'dissolved-groups': stats.dissolvedGroups
    };

    for (const [id, value] of Object.entries(elements)) {
        const element = document.getElementById(id);
        if (element) {
            element.textContent = formatNumber(value);
            element.classList.add('updated');
            setTimeout(function () {
                element.classList.remove('updated');
            }, 500);
        }
    }

    // 群组统计接口没有总用户数，占比必须用单独查到的总用户。
    const userTotal = Number(totalUsers);
    updateTodayActiveCount('today-active-users', stats.todayActiveUsers, Number.isFinite(userTotal) ? userTotal : 0, 'today-active-users-ratio');
    updateTodayActiveCount('today-active-groups', stats.todayActiveGroups, stats.totalGroups, 'today-active-groups-ratio');

    updateDashboardStatusRing(stats);
}

/**
 * 更新白名单统计显示
 */
function updateWhitelistStats(stats) {
    const elements = {
        'total-whitelist': stats.totalWhitelist,
        'active-whitelist': stats.activeWhitelist,
        'disabled-whitelist': stats.disabledWhitelist
    };
    
    for (const [id, value] of Object.entries(elements)) {
        const element = document.getElementById(id);
        if (element) {
            element.textContent = formatNumber(value);
            element.classList.add('updated');
            setTimeout(() => element.classList.remove('updated'), 500);
        }
    }
}

/**
 * 更新今日群聊数量
 */
function updateTodayGroupCount(count) {
    const element = document.getElementById('today-groups');
    if (element) {
        element.textContent = formatNumber(count);
        element.classList.add('updated');
        setTimeout(() => element.classList.remove('updated'), 500);
    }
}

/**
 * 更新总用户数
 */
function updateTotalUserCount(count) {
    const element = document.getElementById('total-users');
    if (element) {
        element.textContent = formatNumber(count);
        element.classList.add('updated');
        setTimeout(() => element.classList.remove('updated'), 500);
    }
}

/**
 * 更新今日新增用户数
 */
function updateTodayNewUserCount(count) {
    const element = document.getElementById('today-new-users');
    if (element) {
        element.textContent = formatNumber(count);
        element.classList.add('updated');
        setTimeout(() => element.classList.remove('updated'), 500);
    }
}

/**
 * 更新每日活跃指标及占比
 */
function updateTodayActiveCount(countId, count, total, ratioId) {
    const el = document.getElementById(countId);
    if (el) {
        el.textContent = formatNumber(count);
        el.classList.add('updated');
        setTimeout(() => el.classList.remove('updated'), 500);
    }
    const ratioEl = document.getElementById(ratioId);
    if (ratioEl) {
        const pct = total > 0 ? ((count / total) * 100).toFixed(1) : '0.0';
        ratioEl.textContent = '占比 ' + pct + '%';
    }
}

/**
 * 更新最后更新时间
 */
function updateLastUpdateTime() {
    const lastUpdateElement = document.getElementById('last-update');
    if (lastUpdateElement && appState.lastUpdate) {
        lastUpdateElement.textContent = formatTimestamp(appState.lastUpdate.getTime());
    }
    const dashSync = document.getElementById('dash-last-update');
    if (dashSync && appState.lastUpdate) {
        dashSync.innerHTML =
            '<i class="far fa-clock"></i> 已同步 ' + formatTimestamp(appState.lastUpdate.getTime());
    }
}

/**
 * 刷新统计数据
 */
async function refreshStats() {
    await refreshCurrent();
}

const REFRESH_MODE_KEY = 'chatadminRefreshMode';
const AUTO_REFRESH_MS = 60000;
let autoRefreshTimer = null;
let refreshInFlight = false;

function getRefreshMode() {
    return localStorage.getItem(REFRESH_MODE_KEY) === 'auto' ? 'auto' : 'manual';
}

function setRefreshMode(mode) {
    localStorage.setItem(REFRESH_MODE_KEY, mode === 'auto' ? 'auto' : 'manual');
    applyRefreshMode();
}

function applyRefreshMode() {
    const mode = getRefreshMode();
    const manualBtn = document.getElementById('ca-refresh-manual');
    const autoBtn = document.getElementById('ca-refresh-auto');
    if (manualBtn) manualBtn.classList.toggle('is-on', mode === 'manual');
    if (autoBtn) autoBtn.classList.toggle('is-on', mode === 'auto');
    if (autoRefreshTimer) {
        clearInterval(autoRefreshTimer);
        autoRefreshTimer = null;
    }
    if (mode !== 'auto') return;
    autoRefreshTimer = setInterval(function () {
        if (document.visibilityState === 'hidden') return;
        if (!window.isLoggedIn || !window.isLoggedIn()) return;
        refreshCurrent();
    }, AUTO_REFRESH_MS);
}

window.setRefreshMode = setRefreshMode;

async function refreshCurrent() {
    if (refreshInFlight) return;
    refreshInFlight = true;
    const refreshBtn = document.getElementById('ca-refresh');
    if (refreshBtn) {
        refreshBtn.classList.add('is-busy');
        refreshBtn.disabled = true;
    }

    try {
        const section = appState.currentSection;
        if (section === 'activity' && typeof loadDailyActiveHistory === 'function') {
            await loadDailyActiveHistory();
        } else if (section === 'groups' && typeof loadGroupsData === 'function') {
            await loadGroupsData();
        } else if (section === 'settings' && typeof loadSettingsData === 'function') {
            loadSettingsData();
        } else if (section === 'loginlogs' && typeof loginLogsAllModule !== 'undefined' && loginLogsAllModule.onSectionShown) {
            loginLogsAllModule.onSectionShown();
        } else if (section === 'audits' && typeof auditsModule !== 'undefined' && auditsModule.onSectionShown) {
            auditsModule.onSectionShown();
        } else if (section === 'userlogin') {
            const searchBtn = document.getElementById('ul-search-btn');
            if (searchBtn) searchBtn.click();
        } else {
            await loadDashboardData();
        }
    } catch (error) {
        console.error('刷新当前页失败:', error);
        showNotification('刷新失败', 'error');
    } finally {
        refreshInFlight = false;
        if (refreshBtn) {
            refreshBtn.classList.remove('is-busy');
            refreshBtn.disabled = false;
        }
    }
}

window.refreshCurrent = refreshCurrent;

/**
 * 初始化设置页面
 */
function initializeSettings() {
    const apiBaseUrlInput = document.getElementById('api-base-url');
    const apiTimeoutInput = document.getElementById('api-timeout');
    
    if (apiBaseUrlInput) {
        apiBaseUrlInput.value = apiClient.baseURL;
    }
    
    if (apiTimeoutInput) {
        apiTimeoutInput.value = apiClient.timeout / 1000;
    }
}

/**
 * 加载设置页面数据
 */
function loadSettingsData() {
    // 设置页面数据已在initializeSettings中加载
}

/**
 * 保存设置
 */
function saveSettings() {
    const apiBaseUrl = document.getElementById('api-base-url').value;
    const apiTimeout = parseInt(document.getElementById('api-timeout').value) * 1000;
    
    // 验证输入
    if (!apiBaseUrl) {
        showNotification('API基础URL不能为空', 'error');
        return;
    }
    
    if (isNaN(apiTimeout) || apiTimeout < 1000) {
        showNotification('超时时间必须大于1秒', 'error');
        return;
    }
    
    try {
        // 更新API客户端配置
        apiClient.updateConfig(apiBaseUrl, apiTimeout);
        
        // 保存到本地存储
        storage.set('appSettings', {
            apiBaseURL: apiBaseUrl,
            apiTimeout: apiTimeout
        });
        
        showNotification('设置保存成功', 'success');
        
        // 测试连接
        testConnection();
        
    } catch (error) {
        console.error('保存设置失败:', error);
        showNotification('保存设置失败', 'error');
    }
}

/**
 * 测试API连接
 */
async function testConnection() {
    try {
        showLoading();
        await apiClient.getGroupStats();
        showNotification('API连接测试成功', 'success');
    } catch (error) {
        showNotification('API连接测试失败: ' + error.message, 'error');
    } finally {
        hideLoading();
    }
}

/**
 * 显示模态框
 */
function showModal(modalId) {
    const modal = document.getElementById(modalId);
    if (modal) {
        modal.style.display = 'block';
        document.body.style.overflow = 'hidden';
    }
}

/**
 * 关闭模态框
 */
function closeModal(modalId) {
    const modal = document.getElementById(modalId);
    if (modal) {
        modal.style.display = 'none';
        document.body.style.overflow = 'auto';
        
        // 清除表单
        const form = modal.querySelector('form');
        if (form) {
            form.reset();
            clearFormErrors();
        }
    }
}

/**
 * 显示群组详情
 */
async function showGroupDetail(groupId) {
    // 确保groupId是字符串，避免精度丢失
    const groupIdStr = String(groupId);
    try {
        showLoading();
        const response = await apiClient.getGroupInfo(groupIdStr);
        
        const content = document.getElementById('group-detail-content');
        if (content) {
            const group = response.data;
            content.innerHTML = `
                <div class="group-detail">
                    <div class="detail-tabs">
                        <button class="tab-btn active" onclick="switchGroupDetailTab('info', '${groupIdStr}', event)">基本信息</button>
                        <button class="tab-btn" onclick="switchGroupDetailTab('members', '${groupIdStr}', event)">群成员</button>
                    </div>
                    <div id="group-detail-info" class="tab-content active">
                        <div class="detail-row">
                            <label>群组ID:</label>
                            <span>${formatLargeNumber(group.groupId)}</span>
                        </div>
                        <div class="detail-row">
                            <label>群组名称:</label>
                            <span>${group.groupName}</span>
                        </div>
                        <div class="detail-row">
                            <label>群组标识:</label>
                            <span>${group.groupMarkId}</span>
                        </div>
                        <div class="detail-row">
                            <label>当前成员数:</label>
                            <span>${formatNumber(group.groupMemberNum)}</span>
                        </div>
                        <div class="detail-row">
                            <label>最大成员数:</label>
                            <span>${formatNumber(group.groupMaximum)}</span>
                        </div>
                        <div class="detail-row">
                            <label>群组状态:</label>
                            <span>${getStatusBadge(group.groupStatus, 'group')}</span>
                        </div>
                        <div class="detail-row">
                            <label>群主ID:</label>
                            <span>${group.groupOwnerId}</span>
                        </div>
                        <div class="detail-row">
                            <label>群组介绍:</label>
                            <span>${group.groupIntroduce || '无'}</span>
                        </div>
                        <div class="detail-row">
                            <label>创建时间:</label>
                            <span>${formatTimestamp(group.groupCreateTime)}</span>
                        </div>
                        <div class="detail-row">
                            <label>更新时间:</label>
                            <span>${formatTimestamp(group.groupUpdateTime)}</span>
                        </div>
                    </div>
                    <div id="group-detail-members" class="tab-content">
                        <div class="loading-members">
                            <i class="fas fa-spinner fa-spin"></i>
                            <p>加载群成员中...</p>
                        </div>
                    </div>
                </div>
            `;
        }
        
        showModal('group-detail-modal');
        
        // 默认加载成员列表（延迟加载，提升体验）
        setTimeout(() => {
            loadGroupMembers(groupIdStr);
        }, 100);
        
    } catch (error) {
        console.error('加载群组详情失败:', error);
        showNotification('加载群组详情失败', 'error');
    } finally {
        hideLoading();
    }
}

/**
 * 切换群组详情标签页
 */
function switchGroupDetailTab(tabName, groupId, event) {
    // 切换标签按钮状态
    const tabBtns = document.querySelectorAll('#group-detail-content .tab-btn');
    tabBtns.forEach(btn => btn.classList.remove('active'));
    
    // 使用传入的事件对象或 window.event
    const evt = event || window.event;
    if (evt && evt.target) {
        evt.target.classList.add('active');
    } else {
        // 如果无法获取事件对象，通过 tabName 查找按钮
        tabBtns.forEach(btn => {
            if (btn.textContent.includes(tabName === 'info' ? '基本信息' : '群成员')) {
                btn.classList.add('active');
            }
        });
    }
    
    // 切换内容区域
    const tabContents = document.querySelectorAll('#group-detail-content .tab-content');
    tabContents.forEach(content => content.classList.remove('active'));
    
    const targetContent = document.getElementById(`group-detail-${tabName}`);
    if (targetContent) {
        targetContent.classList.add('active');
        
        // 如果切换到成员标签且还未加载，则加载成员列表
        if (tabName === 'members') {
            const loadingEl = targetContent.querySelector('.loading-members');
            if (loadingEl || targetContent.innerHTML.trim() === '') {
                loadGroupMembers(groupId);
            }
        }
    }
}

const MEMBER_PAGE_SIZE = 10;

function goMemberPage(page) {
    const membersContent = document.getElementById('group-detail-members');
    if (!membersContent || !membersContent.dataset.groupId) return;
    loadGroupMembers(membersContent.dataset.groupId, page);
}

/**
 * 加载群成员列表
 */
async function loadGroupMembers(groupId, page) {
    const membersContent = document.getElementById('group-detail-members');
    if (!membersContent) return;
    const currentPage = Math.max(1, Number(page) || 1);
    membersContent.dataset.groupId = String(groupId);

    try {
        const response = await apiClient.getGroupMembers(groupId, {
            page: currentPage,
            pageSize: MEMBER_PAGE_SIZE,
        });
        const payload = response.data || {};
        const members = payload.list || [];
        const total = Number(payload.total || 0);
        const pageSize = Number(payload.pageSize) || MEMBER_PAGE_SIZE;
        const totalPages = Math.max(1, Math.ceil(total / pageSize));

        if (total === 0) {
            membersContent.innerHTML = `
                <div class="empty-state">
                    <i class="fas fa-users"></i>
                    <p>该群组暂无成员</p>
                </div>
            `;
            return;
        }
        
        // 格式化成员类型
        const getMemberTypeText = (type) => {
            switch(type) {
                case 2: return '<span class="member-type owner">群主</span>';
                case 1: return '<span class="member-type admin">管理员</span>';
                case 0: return '<span class="member-type member">群员</span>';
                default: return '<span class="member-type">未知</span>';
            }
        };
        
        // 格式化禁言状态
        const getMuteStatus = (muteTime) => {
            if (!muteTime || muteTime === 0) return '';
            const now = Date.now();
            // 检查是否为永久禁言（最大int64值）
            if (muteTime === 9223372036854775807 || muteTime === '9223372036854775807') {
                return '<span class="mute-status permanent">永久禁言</span>';
            }
            // 检查是否还在禁言期内（假设时间戳是毫秒）
            if (muteTime > now) {
                return `<span class="mute-status">禁言至 ${formatTimestamp(muteTime)}</span>`;
            }
            return '';
        };
        
        membersContent.innerHTML = `
            <div class="members-header">
                <h4>群成员列表 (${total}人)</h4>
            </div>
            <div class="table-container">
                <table class="data-table">
                    <thead>
                        <tr>
                            <th>成员ID</th>
                            <th>成员昵称</th>
                            <th>角色</th>
                            <th>加入时间</th>
                            <th>状态</th>
                        </tr>
                    </thead>
                    <tbody>
                        ${members.map(member => `
                            <tr>
                                <td>${member.groupMemberId}</td>
                                <td>${member.groupMemberName || '-'}</td>
                                <td>${getMemberTypeText(member.groupMemberType)}</td>
                                <td>${formatTimestamp(member.groupMemberJoinTime)}</td>
                                <td>${getMuteStatus(member.groupMemberMuteTime)}</td>
                            </tr>
                        `).join('')}
                    </tbody>
                </table>
            </div>
            <div class="members-pagination">${createPagination(currentPage, totalPages, 'goMemberPage')}</div>
        `;
        
    } catch (error) {
        console.error('加载群成员列表失败:', error);
        membersContent.innerHTML = `
            <div class="error-state">
                <i class="fas fa-exclamation-circle"></i>
                <p>加载群成员列表失败: ${error.message || '未知错误'}</p>
                <button class="btn btn-primary btn-sm" onclick="loadGroupMembers('${groupId}')">
                    <i class="fas fa-redo"></i> 重试
                </button>
            </div>
        `;
    }
}

/**
 * 导出数据
 */
function exportData(type) {
    switch(type) {
        case 'groups':
            exportGroupsData();
            break;
        case 'whitelist':
            exportWhitelistData();
            break;
        default:
            showNotification('未知的导出类型', 'error');
    }
}

/**
 * 导出群组数据
 */
async function exportGroupsData() {
    try {
        showLoading();
        const response = await apiClient.getGroupList({
            page: 1,
            pageSize: 1000 // 导出所有数据
        });
        
        const groups = response.data.list.map(group => ({
            '群组ID': group.groupId,
            '群组名称': group.groupName,
            '群组标识': group.groupMarkId,
            '当前成员数': group.groupMemberNum,
            '最大成员数': group.groupMaximum,
            '群组状态': group.groupStatus === 0 ? '正常' : group.groupStatus === 1 ? '封禁' : '解散',
            '群主ID': group.groupOwnerId,
            '创建时间': formatTimestamp(group.groupCreateTime),
            '更新时间': formatTimestamp(group.groupUpdateTime)
        }));
        
        exportToCSV(groups, '群组数据');
        
    } catch (error) {
        console.error('导出群组数据失败:', error);
        showNotification('导出群组数据失败', 'error');
    } finally {
        hideLoading();
    }
}

/**
 * 导出白名单数据
 */
async function exportWhitelistData() {
    try {
        showLoading();
        const response = await apiClient.getWhitelistList({
            page: 1,
            pageSize: 1000 // 导出所有数据
        });
        
        const whitelist = response.data.list.map(item => ({
            'ID': item.id,
            '用户地址': item.address,
            '备注': item.remark,
            '最大成员数': item.maxMembers,
            '状态': item.status === 1 ? '启用' : '禁用',
            '授权人': item.createdBy,
            '创建时间': formatTimestamp(item.createdAt),
            '更新时间': formatTimestamp(item.updatedAt)
        }));
        
        exportToCSV(whitelist, '白名单数据');
        
    } catch (error) {
        console.error('导出白名单数据失败:', error);
        showNotification('导出白名单数据失败', 'error');
    } finally {
        hideLoading();
    }
}

// 全局函数，供HTML调用
window.showSection = showSection;
window.loadServiceConfig = loadServiceConfig;
window.refreshStats = refreshStats;
window.saveSettings = saveSettings;
window.showModal = showModal;
window.closeModal = closeModal;
window.showGroupDetail = showGroupDetail;
window.switchGroupDetailTab = switchGroupDetailTab;
window.loadGroupMembers = loadGroupMembers;
window.exportData = exportData;
