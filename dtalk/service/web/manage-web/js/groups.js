/**
 * 群组管理模块
 * 处理群组相关的所有功能
 */

// 群组管理状态
const groupsState = {
    currentPage: 1,
    pageSize: 10,
    totalPages: 1,
    totalCount: 0,
    groups: [],
    loading: false
};

/**
 * 加载群组数据
 */
async function loadGroupsData() {
    if (window.requireLogin && !window.requireLogin()) {
        // 渲染占位
        const tbody = document.getElementById('groups-table-body');
        if (tbody) {
            tbody.innerHTML = `
                <tr>
                    <td colspan="7" class="empty-state">
                        <i class="fas fa-lock"></i>
                        <p>请先登录后查看群组数据</p>
                    </td>
                </tr>
            `;
        }
        const pagination = document.getElementById('groups-pagination');
        if (pagination) pagination.innerHTML = '';
        return;
    }
    if (groupsState.loading) return;
    
    try {
        groupsState.loading = true;
        showLoading();
        
        const params = {
            page: groupsState.currentPage,
            pageSize: groupsState.pageSize
        };
        
        // 添加搜索条件
        if (appState.searchKeyword) {
            params.keyword = appState.searchKeyword;
        }
        
        // 添加状态筛选
        if (appState.statusFilter) {
            params.status = appState.statusFilter;
        }
        
        const response = await apiClient.getGroupList(params);
        groupsState.groups = response.data.list;
        groupsState.totalCount = response.data.total;
        groupsState.totalPages = Math.ceil(groupsState.totalCount / groupsState.pageSize);
        
        renderGroupsTable();
        renderGroupsPagination();
        
    } catch (error) {
        console.error('加载群组数据失败:', error);
        showNotification('加载群组数据失败', 'error');
    } finally {
        groupsState.loading = false;
        hideLoading();
    }
}

/**
 * 渲染群组表格
 */
function renderGroupsTable() {
    const tbody = document.getElementById('groups-table-body');
    if (!tbody) return;
    
    if (groupsState.groups.length === 0) {
        tbody.innerHTML = `
            <tr>
                <td colspan="7" class="empty-state">
                    <i class="fas fa-inbox"></i>
                    <p>暂无群组数据</p>
                </td>
            </tr>
        `;
        return;
    }
    
    tbody.innerHTML = groupsState.groups.map((group, index) => `
        <tr>
            <td>${formatLargeNumber(group.groupId)}</td>
            <td>
                <div class="group-name">
                    <strong>${group.groupName}</strong>
                    <br>
                    <small class="text-muted">${group.groupMarkId}</small>
                </div>
            </td>
            <td class="number-cell">${formatNumber(group.groupMemberNum)}</td>
            <td class="number-cell">${formatNumber(group.groupMaximum)}</td>
            <td class="status-cell">${getStatusBadge(group.groupStatus, 'group')}</td>
            <td>${formatTimestamp(group.groupCreateTime)}</td>
            <td class="action-cell">
                <button class="btn btn-primary btn-sm" onclick="showGroupDetail('${group.groupId}')" title="查看详情">
                    <i class="fas fa-eye"></i>
                </button>
                <button class="btn btn-warning btn-sm" onclick="editGroup('${group.groupId}')" title="编辑">
                    <i class="fas fa-edit"></i>
                </button>
                <button class="btn btn-danger btn-sm" onclick="deleteGroup('${group.groupId}')" title="删除">
                    <i class="fas fa-trash"></i>
                </button>
            </td>
        </tr>
    `).join('');
}

/**
 * 渲染群组分页
 */
function renderGroupsPagination() {
    const pagination = document.getElementById('groups-pagination');
    if (!pagination) return;
    
    pagination.innerHTML = createPagination(
        groupsState.currentPage,
        groupsState.totalPages,
        'changeGroupsPage'
    );
}

/**
 * 切换群组页面
 */
function changeGroupsPage(page) {
    if (page < 1 || page > groupsState.totalPages || page === groupsState.currentPage) {
        return;
    }
    
    groupsState.currentPage = page;
    loadGroupsData();
}

/**
 * 搜索群组
 */
function searchGroups() {
    const searchInput = document.getElementById('group-search');
    if (searchInput) {
        appState.searchKeyword = searchInput.value;
    }
    
    const statusFilter = document.getElementById('group-status-filter');
    if (statusFilter) {
        appState.statusFilter = statusFilter.value;
    }
    
    groupsState.currentPage = 1;
    loadGroupsData();
}

/**
 * 刷新群组数据
 */
function refreshGroups() {
    groupsState.currentPage = 1;
    appState.searchKeyword = '';
    appState.statusFilter = '';
    
    // 重置搜索框
    const searchInput = document.getElementById('group-search');
    if (searchInput) {
        searchInput.value = '';
    }
    
    // 重置状态筛选
    const statusFilter = document.getElementById('group-status-filter');
    if (statusFilter) {
        statusFilter.value = '';
    }
    
    loadGroupsData();
}

/**
 * 编辑群组
 */
async function editGroup(groupId) {
    // 确保groupId是字符串，避免精度丢失
    const groupIdStr = String(groupId);
    try {
        showLoading();
        const response = await apiClient.getGroupInfo(groupIdStr);
        const group = response.data;
        
        // 创建编辑表单
        const formHtml = `
            <div class="edit-group-form">
                <div class="form-group">
                    <label>群组名称</label>
                    <input type="text" id="edit-group-name" value="${group.groupName}" class="form-input">
                </div>
                <div class="form-group">
                    <label>最大成员数</label>
                    <input type="number" id="edit-group-maximum" value="${group.groupMaximum}" min="1" class="form-input">
                </div>
                <div class="form-group">
                    <label>群组状态</label>
                    <select id="edit-group-status" class="form-input">
                        <option value="0" ${group.groupStatus === 0 ? 'selected' : ''}>正常</option>
                        <option value="1" ${group.groupStatus === 1 ? 'selected' : ''}>封禁</option>
                        <option value="2" ${group.groupStatus === 2 ? 'selected' : ''}>解散</option>
                    </select>
                </div>
            </div>
        `;
        
        // 显示编辑模态框
        showEditGroupModal(groupIdStr, formHtml);
        
    } catch (error) {
        console.error('加载群组信息失败:', error);
        showNotification('加载群组信息失败', 'error');
    } finally {
        hideLoading();
    }
}

/**
 * 显示编辑群组模态框
 */
function showEditGroupModal(groupId, formHtml) {
    // 创建模态框HTML
    const modalHtml = `
        <div id="edit-group-modal" class="modal" style="display: block;">
            <div class="modal-content">
                <div class="modal-header">
                    <h3>编辑群组</h3>
                    <button class="modal-close" onclick="closeEditGroupModal()">&times;</button>
                </div>
                <div class="modal-body">
                    ${formHtml}
                </div>
                <div class="modal-footer">
                    <button class="btn btn-secondary" onclick="closeEditGroupModal()">取消</button>
                    <button class="btn btn-primary" onclick="submitEditGroup('${groupId}')">保存</button>
                </div>
            </div>
        </div>
    `;
    
    // 添加到页面
    document.body.insertAdjacentHTML('beforeend', modalHtml);
    document.body.style.overflow = 'hidden';
}

/**
 * 关闭编辑群组模态框
 */
function closeEditGroupModal() {
    const modal = document.getElementById('edit-group-modal');
    if (modal) {
        modal.remove();
        document.body.style.overflow = 'auto';
    }
}

/**
 * 提交编辑群组
 */
async function submitEditGroup(groupId) {
    // 确保groupId是字符串，避免精度丢失
    const groupIdStr = String(groupId);
    const name = document.getElementById('edit-group-name').value;
    const maximum = parseInt(document.getElementById('edit-group-maximum').value);
    const status = parseInt(document.getElementById('edit-group-status').value);
    
    // 验证表单
    const errors = {};
    if (!name.trim()) {
        errors.name = '群组名称不能为空';
    }
    if (isNaN(maximum) || maximum < 1) {
        errors.maximum = '最大成员数必须大于0';
    }
    if (isNaN(status) || status < 0 || status > 2) {
        errors.status = '群组状态无效';
    }
    
    if (Object.keys(errors).length > 0) {
        showFormErrors(errors);
        return;
    }
    
    try {
        showLoading();
        
        // 更新群组信息
        await apiClient.updateGroupName(groupIdStr, name);
        await apiClient.updateGroupMaximum(groupIdStr, maximum);
        await apiClient.updateGroupStatus(groupIdStr, status);
        
        showNotification('群组更新成功', 'success');
        closeEditGroupModal();
        loadGroupsData();
        
    } catch (error) {
        console.error('更新群组失败:', error);
        showNotification('更新群组失败: ' + error.message, 'error');
    } finally {
        hideLoading();
    }
}

/**
 * 删除群组
 */
async function deleteGroup(groupId) {
    // 确保groupId是字符串，避免精度丢失
    const groupIdStr = String(groupId);
    if (!confirm('确定要删除这个群组吗？此操作不可撤销。')) {
        return;
    }
    
    try {
        showLoading();
        
        // 这里应该调用删除群组的API，但当前API中没有删除接口
        // 所以这里只是演示，实际应该根据API调整
        showNotification('删除功能暂未实现', 'warning');
        
    } catch (error) {
        console.error('删除群组失败:', error);
        showNotification('删除群组失败: ' + error.message, 'error');
    } finally {
        hideLoading();
    }
}

/**
 * 根据状态查询群组
 */
async function getGroupsByStatus(status) {
    try {
        showLoading();
        
        const params = {
            page: 1,
            pageSize: 50
        };
        
        const response = await apiClient.getGroupsByStatus(status, params);
        const groups = response.data.list;
        
        // 显示结果
        showGroupsByStatusModal(status, groups);
        
    } catch (error) {
        console.error('查询群组失败:', error);
        showNotification('查询群组失败', 'error');
    } finally {
        hideLoading();
    }
}

/**
 * 显示按状态查询的群组结果
 */
function showGroupsByStatusModal(status, groups) {
    const statusText = {
        0: '正常',
        1: '封禁',
        2: '解散'
    };
    
    const modalHtml = `
        <div id="groups-by-status-modal" class="modal" style="display: block;">
            <div class="modal-content large">
                <div class="modal-header">
                    <h3>${statusText[status]}状态群组 (${groups.length}个)</h3>
                    <button class="modal-close" onclick="closeGroupsByStatusModal()">&times;</button>
                </div>
                <div class="modal-body">
                    <div class="table-container">
                        <table class="data-table">
                            <thead>
                                <tr>
                                    <th>群组ID</th>
                                    <th>群组名称</th>
                                    <th>成员数</th>
                                    <th>最大人数</th>
                                    <th>创建时间</th>
                                    <th>操作</th>
                                </tr>
                            </thead>
                            <tbody>
                                ${groups.map(group => `
                                    <tr>
                                        <td>${group.groupId}</td>
                                        <td>${group.groupName}</td>
                                        <td class="number-cell">${formatNumber(group.groupMemberNum)}</td>
                                        <td class="number-cell">${formatNumber(group.groupMaximum)}</td>
                                        <td>${formatTimestamp(group.groupCreateTime)}</td>
                                        <td>
                                            <button class="btn btn-primary btn-sm" onclick="showGroupDetail(${group.groupId})">
                                                <i class="fas fa-eye"></i>
                                            </button>
                                        </td>
                                    </tr>
                                `).join('')}
                            </tbody>
                        </table>
                    </div>
                </div>
                <div class="modal-footer">
                    <button class="btn btn-secondary" onclick="closeGroupsByStatusModal()">关闭</button>
                </div>
            </div>
        </div>
    `;
    
    document.body.insertAdjacentHTML('beforeend', modalHtml);
    document.body.style.overflow = 'hidden';
}

/**
 * 关闭按状态查询的群组模态框
 */
function closeGroupsByStatusModal() {
    const modal = document.getElementById('groups-by-status-modal');
    if (modal) {
        modal.remove();
        document.body.style.overflow = 'auto';
    }
}

// 全局函数，供HTML调用
window.loadGroupsData = loadGroupsData;
window.changeGroupsPage = changeGroupsPage;
window.searchGroups = searchGroups;
window.refreshGroups = refreshGroups;
window.editGroup = editGroup;
window.deleteGroup = deleteGroup;
window.getGroupsByStatus = getGroupsByStatus;
window.closeEditGroupModal = closeEditGroupModal;
window.submitEditGroup = submitEditGroup;
window.closeGroupsByStatusModal = closeGroupsByStatusModal;
