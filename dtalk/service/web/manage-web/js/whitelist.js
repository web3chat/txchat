/**
 * 白名单管理模块
 * 处理白名单相关的所有功能
 */

// 白名单管理状态
const whitelistState = {
    currentPage: 1,
    pageSize: 10,
    totalPages: 1,
    totalCount: 0,
    whitelist: [],
    loading: false
};

/**
 * 加载白名单数据
 */
async function loadWhitelistData() {
    if (window.requireLogin && !window.requireLogin()) {
        const tbody = document.getElementById('whitelist-table-body');
        if (tbody) {
            tbody.innerHTML = `
                <tr>
                    <td colspan="9" class="empty-state">
                        <i class="fas fa-lock"></i>
                        <p>请先登录后查看白名单数据</p>
                    </td>
                </tr>
            `;
        }
        const pagination = document.getElementById('whitelist-pagination');
        if (pagination) pagination.innerHTML = '';
        return;
    }
    if (whitelistState.loading) return;
    
    try {
        whitelistState.loading = true;
        showLoading();
        
        const params = {
            page: whitelistState.currentPage,
            pageSize: whitelistState.pageSize
        };
        
        const response = await apiClient.getWhitelistList(params);
        whitelistState.whitelist = response.data.list;
        whitelistState.totalCount = response.data.total;
        whitelistState.totalPages = Math.ceil(whitelistState.totalCount / whitelistState.pageSize);
        
        renderWhitelistTable();
        renderWhitelistPagination();
        
    } catch (error) {
        console.error('加载白名单数据失败:', error);
        showNotification('加载白名单数据失败', 'error');
    } finally {
        whitelistState.loading = false;
        hideLoading();
    }
}

/**
 * 渲染白名单表格
 */
function renderWhitelistTable() {
    const tbody = document.getElementById('whitelist-table-body');
    if (!tbody) return;
    
    if (whitelistState.whitelist.length === 0) {
        tbody.innerHTML = `
            <tr>
                <td colspan="9" class="empty-state">
                    <i class="fas fa-inbox"></i>
                    <p>暂无白名单数据</p>
                </td>
            </tr>
        `;
        return;
    }
    
    tbody.innerHTML = whitelistState.whitelist.map(item => `
        <tr>
            <td>${item.id}</td>
            <td>
                <div class="address-cell">
                    <code>${item.address}</code>
                </div>
            </td>
            <td>${item.remark || '-'}</td>
            <td class="number-cell">${formatNumber(item.maxMembers)}</td>
            <td class="number-cell">${formatNumber(item.maxGroups || 10)}</td>
            <td class="status-cell">${getStatusBadge(item.status, 'whitelist')}</td>
            <td>${item.createdBy}</td>
            <td>${formatTimestamp(item.createdAt)}</td>
            <td class="action-cell">
                <button class="btn btn-primary btn-sm" onclick="viewWhitelist(${item.id})" title="查看详情">
                    <i class="fas fa-eye"></i>
                </button>
                <button class="btn btn-info btn-sm" onclick="viewGroupsByAddress('${item.address}')" title="查看创建的群聊">
                    <i class="fas fa-users"></i>
                </button>
                <button class="btn btn-warning btn-sm" onclick="editWhitelist(${item.id})" title="编辑">
                    <i class="fas fa-edit"></i>
                </button>
                <button class="btn btn-danger btn-sm" onclick="deleteWhitelist(${item.id})" title="删除">
                    <i class="fas fa-trash"></i>
                </button>
            </td>
        </tr>
    `).join('');
}

/**
 * 渲染白名单分页
 */
function renderWhitelistPagination() {
    const pagination = document.getElementById('whitelist-pagination');
    if (!pagination) return;
    
    pagination.innerHTML = createPagination(
        whitelistState.currentPage,
        whitelistState.totalPages,
        'changeWhitelistPage'
    );
}

/**
 * 切换白名单页面
 */
function changeWhitelistPage(page) {
    if (page < 1 || page > whitelistState.totalPages || page === whitelistState.currentPage) {
        return;
    }
    
    whitelistState.currentPage = page;
    loadWhitelistData();
}

/**
 * 刷新白名单数据
 */
function refreshWhitelist() {
    whitelistState.currentPage = 1;
    loadWhitelistData();
}

/**
 * 显示添加白名单模态框
 */
function showAddWhitelistModal() {
    showModal('add-whitelist-modal');
    // 自动填充授权人为当前登录地址，并设为只读
    try {
        const addr = localStorage.getItem('adminAddress') || '';
        const input = document.getElementById('whitelist-created-by');
        if (input) {
            input.value = addr;
            input.readOnly = true;
            input.placeholder = addr ? '' : '请先登录后再添加';
        }
    } catch (e) {}
}

/**
 * 提交添加白名单
 */
async function submitAddWhitelist() {
    const form = document.getElementById('add-whitelist-form');
    const formData = new FormData(form);
    
    const data = {
        address: document.getElementById('whitelist-address').value,
        remark: document.getElementById('whitelist-remark').value,
        maxMembers: parseInt(document.getElementById('whitelist-max-members').value),
        maxGroups: parseInt(document.getElementById('whitelist-max-groups').value),
        // 始终使用当前登录地址作为授权人
        createdBy: (function() {
            const addr = localStorage.getItem('adminAddress');
            return addr || '';
        })()
    };
    
    // 验证表单
    const rules = {
        address: { required: true, label: '用户地址' },
        maxMembers: { required: true, min: 1, label: '最大成员数' },
        maxGroups: { required: true, min: 1, label: '最大创建群聊个数' },
        createdBy: { required: true, label: '授权人' }
    };
    
    if (window.requireLogin && !window.requireLogin()) {
        return;
    }

    const validation = validateForm(data, rules);
    if (!validation.isValid) {
        showFormErrors(validation.errors);
        return;
    }
    
    try {
        showLoading();
        await apiClient.addWhitelist(data);
        
        showNotification('白名单添加成功', 'success');
        closeModal('add-whitelist-modal');
        loadWhitelistData();
        
    } catch (error) {
        console.error('添加白名单失败:', error);
        showNotification('添加白名单失败: ' + error.message, 'error');
    } finally {
        hideLoading();
    }
}

/**
 * 查看白名单详情
 */
async function viewWhitelist(whitelistId) {
    try {
        showLoading();
        const response = await apiClient.getWhitelistInfo(whitelistId);
        const item = response.data;
        
        const detailHtml = `
            <div class="whitelist-detail">
                <div class="detail-row">
                    <label>ID:</label>
                    <span>${item.id}</span>
                </div>
                <div class="detail-row">
                    <label>用户地址:</label>
                    <span><code>${item.address}</code></span>
                </div>
                <div class="detail-row">
                    <label>备注:</label>
                    <span>${item.remark || '无'}</span>
                </div>
                <div class="detail-row">
                    <label>最大成员数:</label>
                    <span>${formatNumber(item.maxMembers)}</span>
                </div>
                <div class="detail-row">
                    <label>最大创建群聊个数:</label>
                    <span>${formatNumber(item.maxGroups || 10)}</span>
                </div>
                <div class="detail-row">
                    <label>状态:</label>
                    <span>${getStatusBadge(item.status, 'whitelist')}</span>
                </div>
                <div class="detail-row">
                    <label>授权人:</label>
                    <span>${item.createdBy}</span>
                </div>
                <div class="detail-row">
                    <label>创建时间:</label>
                    <span>${formatTimestamp(item.createdAt)}</span>
                </div>
                <div class="detail-row">
                    <label>更新时间:</label>
                    <span>${formatTimestamp(item.updatedAt)}</span>
                </div>
            </div>
        `;
        
        // 显示详情模态框
        showWhitelistDetailModal(detailHtml);
        
    } catch (error) {
        console.error('加载白名单详情失败:', error);
        showNotification('加载白名单详情失败', 'error');
    } finally {
        hideLoading();
    }
}

/**
 * 显示白名单详情模态框
 */
function showWhitelistDetailModal(content) {
    const modalHtml = `
        <div id="whitelist-detail-modal" class="modal" style="display: block;">
            <div class="modal-content">
                <div class="modal-header">
                    <h3>白名单详情</h3>
                    <button class="modal-close" onclick="closeWhitelistDetailModal()">&times;</button>
                </div>
                <div class="modal-body">
                    ${content}
                </div>
                <div class="modal-footer">
                    <button class="btn btn-secondary" onclick="closeWhitelistDetailModal()">关闭</button>
                </div>
            </div>
        </div>
    `;
    
    document.body.insertAdjacentHTML('beforeend', modalHtml);
    document.body.style.overflow = 'hidden';
}

/**
 * 关闭白名单详情模态框
 */
function closeWhitelistDetailModal() {
    const modal = document.getElementById('whitelist-detail-modal');
    if (modal) {
        modal.remove();
        document.body.style.overflow = 'auto';
    }
}

/**
 * 编辑白名单
 */
async function editWhitelist(whitelistId) {
    try {
        showLoading();
        const response = await apiClient.getWhitelistInfo(whitelistId);
        const item = response.data;
        
        // 填充编辑表单
        document.getElementById('edit-whitelist-id').value = item.id;
        document.getElementById('edit-whitelist-address').value = item.address;
        document.getElementById('edit-whitelist-remark').value = item.remark;
        document.getElementById('edit-whitelist-max-members').value = item.maxMembers;
        document.getElementById('edit-whitelist-max-groups').value = item.maxGroups || 10;
        document.getElementById('edit-whitelist-status').value = item.status;
        
        showModal('edit-whitelist-modal');
        
    } catch (error) {
        console.error('加载白名单信息失败:', error);
        showNotification('加载白名单信息失败', 'error');
    } finally {
        hideLoading();
    }
}

/**
 * 提交编辑白名单
 */
async function submitEditWhitelist() {
    const whitelistId = document.getElementById('edit-whitelist-id').value;
    const data = {
        address: document.getElementById('edit-whitelist-address').value,
        remark: document.getElementById('edit-whitelist-remark').value,
        maxMembers: parseInt(document.getElementById('edit-whitelist-max-members').value),
        maxGroups: parseInt(document.getElementById('edit-whitelist-max-groups').value),
        status: parseInt(document.getElementById('edit-whitelist-status').value)
    };
    
    // 验证表单
    const rules = {
        maxMembers: { min: 1, label: '最大成员数' },
        maxGroups: { min: 1, label: '最大创建群聊个数' },
        status: { min: 0, max: 1, label: '状态' }
    };
    
    const validation = validateForm(data, rules);
    if (!validation.isValid) {
        showFormErrors(validation.errors);
        return;
    }
    
    try {
        showLoading();
        await apiClient.updateWhitelist(whitelistId, data);
        
        showNotification('白名单更新成功', 'success');
        closeModal('edit-whitelist-modal');
        loadWhitelistData();
        
    } catch (error) {
        console.error('更新白名单失败:', error);
        showNotification('更新白名单失败: ' + error.message, 'error');
    } finally {
        hideLoading();
    }
}

/**
 * 删除白名单
 */
async function deleteWhitelist(whitelistId) {
    if (!confirm('确定要删除这个白名单吗？此操作不可撤销。')) {
        return;
    }
    
    try {
        showLoading();
        await apiClient.deleteWhitelist(whitelistId);
        
        showNotification('白名单删除成功', 'success');
        loadWhitelistData();
        
    } catch (error) {
        console.error('删除白名单失败:', error);
        showNotification('删除白名单失败: ' + error.message, 'error');
    } finally {
        hideLoading();
    }
}

/**
 * 批量操作白名单
 */
function batchOperationWhitelist(operation) {
    const checkboxes = document.querySelectorAll('#whitelist-table-body input[type="checkbox"]:checked');
    if (checkboxes.length === 0) {
        showNotification('请选择要操作的白名单', 'warning');
        return;
    }
    
    const ids = Array.from(checkboxes).map(cb => cb.value);
    
    switch(operation) {
        case 'enable':
            batchEnableWhitelist(ids);
            break;
        case 'disable':
            batchDisableWhitelist(ids);
            break;
        case 'delete':
            batchDeleteWhitelist(ids);
            break;
        default:
            showNotification('未知的操作类型', 'error');
    }
}

/**
 * 批量启用白名单
 */
async function batchEnableWhitelist(ids) {
    try {
        showLoading();
        
        const promises = ids.map(id => 
            apiClient.updateWhitelist(id, { status: 1 })
        );
        
        await Promise.all(promises);
        
        showNotification(`成功启用 ${ids.length} 个白名单`, 'success');
        loadWhitelistData();
        
    } catch (error) {
        console.error('批量启用白名单失败:', error);
        showNotification('批量启用白名单失败', 'error');
    } finally {
        hideLoading();
    }
}

/**
 * 批量禁用白名单
 */
async function batchDisableWhitelist(ids) {
    try {
        showLoading();
        
        const promises = ids.map(id => 
            apiClient.updateWhitelist(id, { status: 0 })
        );
        
        await Promise.all(promises);
        
        showNotification(`成功禁用 ${ids.length} 个白名单`, 'success');
        loadWhitelistData();
        
    } catch (error) {
        console.error('批量禁用白名单失败:', error);
        showNotification('批量禁用白名单失败', 'error');
    } finally {
        hideLoading();
    }
}

/**
 * 批量删除白名单
 */
async function batchDeleteWhitelist(ids) {
    if (!confirm(`确定要删除选中的 ${ids.length} 个白名单吗？此操作不可撤销。`)) {
        return;
    }
    
    try {
        showLoading();
        
        const promises = ids.map(id => apiClient.deleteWhitelist(id));
        await Promise.all(promises);
        
        showNotification(`成功删除 ${ids.length} 个白名单`, 'success');
        loadWhitelistData();
        
    } catch (error) {
        console.error('批量删除白名单失败:', error);
        showNotification('批量删除白名单失败', 'error');
    } finally {
        hideLoading();
    }
}

/**
 * 搜索白名单
 */
function searchWhitelist() {
    const keyword = document.getElementById('whitelist-search')?.value || '';
    const status = document.getElementById('whitelist-status-filter')?.value || '';
    
    // 这里可以实现搜索功能，暂时只是刷新数据
    whitelistState.currentPage = 1;
    loadWhitelistData();
}

/**
 * 导出白名单数据
 */
async function exportWhitelistData() {
    try {
        showLoading();
        
        const response = await apiClient.getWhitelistList({
            page: 1,
            pageSize: 1000
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
window.loadWhitelistData = loadWhitelistData;
window.changeWhitelistPage = changeWhitelistPage;
window.refreshWhitelist = refreshWhitelist;
window.showAddWhitelistModal = showAddWhitelistModal;
window.submitAddWhitelist = submitAddWhitelist;
window.viewWhitelist = viewWhitelist;
window.editWhitelist = editWhitelist;
window.submitEditWhitelist = submitEditWhitelist;
window.deleteWhitelist = deleteWhitelist;
window.closeWhitelistDetailModal = closeWhitelistDetailModal;
window.batchOperationWhitelist = batchOperationWhitelist;
window.searchWhitelist = searchWhitelist;
window.exportWhitelistData = exportWhitelistData;
window.viewGroupsByAddress = viewGroupsByAddress;
window.closeGroupsByOwnerModal = closeGroupsByOwnerModal;
window.searchByAddress = searchByAddress;
window.clearSearchResult = clearSearchResult;

/**
 * 查看某个地址创建的群聊列表
 */
async function viewGroupsByAddress(address) {
    try {
        showLoading();
        const response = await apiClient.getGroupsByOwner(address, { page: 1, pageSize: 20 });
        const groups = response.data.list || [];
        const total = response.data.total || 0;
        
        if (groups.length === 0) {
            showNotification(`地址 ${address} 尚未创建任何群聊`, 'info');
            return;
        }
        
        const groupsHtml = `
            <div class="groups-by-owner">
                <div class="groups-header">
                    <h4>地址 ${address} 创建的群聊列表</h4>
                    <p>共 ${total} 个群聊</p>
                </div>
                <div class="table-container">
                    <table class="data-table">
                        <thead>
                            <tr>
                                <th>群组ID</th>
                                <th>群组名称</th>
                                <th>成员数</th>
                                <th>最大人数</th>
                                <th>状态</th>
                                <th>创建时间</th>
                            </tr>
                        </thead>
                        <tbody>
                            ${groups.map(group => `
                                <tr>
                                    <td>${group.groupId}</td>
                                    <td>${group.groupName || '-'}</td>
                                    <td>${formatNumber(group.groupMemberNum || 0)}</td>
                                    <td>${formatNumber(group.groupMaximum || 0)}</td>
                                    <td>${getStatusBadge(group.groupStatus || 0, 'group')}</td>
                                    <td>${formatTimestamp(group.groupCreateTime)}</td>
                                </tr>
                            `).join('')}
                        </tbody>
                    </table>
                </div>
            </div>
        `;
        
        // 显示群聊列表模态框
        showGroupsByOwnerModal(groupsHtml);
        
    } catch (error) {
        console.error('查询群聊列表失败:', error);
        showNotification('查询群聊列表失败: ' + error.message, 'error');
    } finally {
        hideLoading();
    }
}

/**
 * 显示地址创建的群聊列表模态框
 */
function showGroupsByOwnerModal(content) {
    const modalHtml = `
        <div id="groups-by-owner-modal" class="modal" style="display: block;">
            <div class="modal-content large">
                <div class="modal-header">
                    <h3>创建的群聊列表</h3>
                    <button class="modal-close" onclick="closeGroupsByOwnerModal()">&times;</button>
                </div>
                <div class="modal-body">
                    ${content}
                </div>
                <div class="modal-footer">
                    <button class="btn btn-secondary" onclick="closeGroupsByOwnerModal()">关闭</button>
                </div>
            </div>
        </div>
    `;
    
    document.body.insertAdjacentHTML('beforeend', modalHtml);
    document.body.style.overflow = 'hidden';
}

/**
 * 关闭地址创建的群聊列表模态框
 */
function closeGroupsByOwnerModal() {
    const modal = document.getElementById('groups-by-owner-modal');
    if (modal) {
        modal.remove();
        document.body.style.overflow = 'auto';
    }
}

/**
 * 根据地址查询白名单信息和创建的群聊
 */
async function searchByAddress() {
    const addressInput = document.getElementById('address-search-input');
    const address = addressInput.value.trim();
    
    if (!address) {
        showNotification('请输入用户地址', 'warning');
        return;
    }
    
    // 验证地址格式
    // 0x地址：以太坊格式地址，42字符（0x + 40个十六进制字符）
    // 1x地址（Base58格式）：比特币地址格式，以1开头，通常34字符（Base58编码）
    const isValidAddress = 
        (address.startsWith('0x') && address.length >= 42 && address.length <= 50) ||
        (address.startsWith('1') && address.length >= 20 && address.length <= 50);
    
    if (!isValidAddress) {
        showNotification('请输入有效的地址（支持0x开头的以太坊地址或1开头的Base58地址）', 'warning');
        return;
    }
    
    try {
        showLoading();
        const resultDiv = document.getElementById('address-search-result');
        resultDiv.style.display = 'block';
        
        // 查询白名单信息
        let whitelistInfo = null;
        try {
            const whitelistResponse = await apiClient.getWhitelistByAddress(address);
            whitelistInfo = whitelistResponse.data;
            displayWhitelistSearchResult(whitelistInfo);
        } catch (error) {
            if (error.message && error.message.includes('404')) {
                displayWhitelistSearchResult(null, '该地址不在白名单中');
            } else {
                displayWhitelistSearchResult(null, '查询白名单信息失败: ' + error.message);
            }
        }
        
        // 查询创建的群聊信息
        try {
            const groupsResponse = await apiClient.getGroupsByOwner(address, { page: 1, pageSize: 50 });
            const groups = groupsResponse.data.list || [];
            const total = groupsResponse.data.total || 0;
            displayGroupsSearchResult(groups, total);
        } catch (error) {
            displayGroupsSearchResult([], 0, '查询群聊信息失败: ' + error.message);
        }
        
    } catch (error) {
        console.error('查询失败:', error);
        showNotification('查询失败: ' + error.message, 'error');
    } finally {
        hideLoading();
    }
}

/**
 * 显示白名单查询结果
 */
function displayWhitelistSearchResult(whitelist, errorMsg = null) {
    const contentDiv = document.getElementById('whitelist-search-content');
    
    if (errorMsg) {
        contentDiv.innerHTML = `
            <div style="padding: 20px; text-align: center; color: #999;">
                <i class="fas fa-info-circle" style="font-size: 2rem; margin-bottom: 10px;"></i>
                <p>${errorMsg}</p>
            </div>
        `;
        return;
    }
    
    if (!whitelist) {
        contentDiv.innerHTML = `
            <div style="padding: 20px; text-align: center; color: #999;">
                <i class="fas fa-info-circle" style="font-size: 2rem; margin-bottom: 10px;"></i>
                <p>该地址不在白名单中</p>
            </div>
        `;
        return;
    }
    
    const statusText = whitelist.status === 1 ? '启用' : '禁用';
    const statusClass = whitelist.status === 1 ? 'status-badge active' : 'status-badge disabled';
    
    contentDiv.innerHTML = `
        <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(250px, 1fr)); gap: 15px;">
            <div style="padding: 15px; background: #f8f9fa; border-radius: 8px;">
                <div style="font-weight: bold; color: #666; margin-bottom: 5px;">ID</div>
                <div style="font-size: 1.1rem; color: #333;">${whitelist.id || '-'}</div>
            </div>
            <div style="padding: 15px; background: #f8f9fa; border-radius: 8px;">
                <div style="font-weight: bold; color: #666; margin-bottom: 5px;">用户地址</div>
                <div style="font-size: 0.9rem; color: #333; word-break: break-all;">
                    <code style="background: #e9ecef; padding: 4px 8px; border-radius: 4px;">${whitelist.address || '-'}</code>
                </div>
            </div>
            <div style="padding: 15px; background: #f8f9fa; border-radius: 8px;">
                <div style="font-weight: bold; color: #666; margin-bottom: 5px;">备注</div>
                <div style="font-size: 1.1rem; color: #333;">${whitelist.remark || '-'}</div>
            </div>
            <div style="padding: 15px; background: #f8f9fa; border-radius: 8px;">
                <div style="font-weight: bold; color: #666; margin-bottom: 5px;">最大成员数</div>
                <div style="font-size: 1.1rem; color: #333;">${whitelist.maxMembers || '-'}</div>
            </div>
            <div style="padding: 15px; background: #f8f9fa; border-radius: 8px;">
                <div style="font-weight: bold; color: #666; margin-bottom: 5px;">最大创建群聊个数</div>
                <div style="font-size: 1.1rem; color: #333;">${whitelist.maxGroups || '-'}</div>
            </div>
            <div style="padding: 15px; background: #f8f9fa; border-radius: 8px;">
                <div style="font-weight: bold; color: #666; margin-bottom: 5px;">状态</div>
                <div><span class="${statusClass}">${statusText}</span></div>
            </div>
            <div style="padding: 15px; background: #f8f9fa; border-radius: 8px;">
                <div style="font-weight: bold; color: #666; margin-bottom: 5px;">授权人</div>
                <div style="font-size: 0.9rem; color: #333; word-break: break-all;">
                    <code style="background: #e9ecef; padding: 4px 8px; border-radius: 4px;">${whitelist.createdBy || '-'}</code>
                </div>
            </div>
            <div style="padding: 15px; background: #f8f9fa; border-radius: 8px;">
                <div style="font-weight: bold; color: #666; margin-bottom: 5px;">创建时间</div>
                <div style="font-size: 0.9rem; color: #333;">${whitelist.createdAt ? formatTimestamp(whitelist.createdAt) : '-'}</div>
            </div>
        </div>
    `;
}

/**
 * 显示群聊查询结果
 */
function displayGroupsSearchResult(groups, total, errorMsg = null) {
    const contentDiv = document.getElementById('groups-search-content');
    
    if (errorMsg) {
        contentDiv.innerHTML = `
            <div style="padding: 20px; text-align: center; color: #f00;">
                <i class="fas fa-exclamation-circle" style="font-size: 2rem; margin-bottom: 10px;"></i>
                <p>${errorMsg}</p>
            </div>
        `;
        return;
    }
    
    if (groups.length === 0) {
        contentDiv.innerHTML = `
            <div style="padding: 20px; text-align: center; color: #999;">
                <i class="fas fa-inbox" style="font-size: 2rem; margin-bottom: 10px;"></i>
                <p>该地址尚未创建任何群聊</p>
            </div>
        `;
        return;
    }
    
    let tableHTML = `
        <div style="margin-bottom: 15px; color: #666;">
            <strong>共找到 ${total} 个群聊（显示前 ${groups.length} 个）</strong>
        </div>
        <div style="overflow-x: auto;">
            <table class="data-table" style="min-width: 1000px;">
                <thead>
                    <tr>
                        <th>群组ID</th>
                        <th>群组名称</th>
                        <th>成员数</th>
                        <th>最大人数</th>
                        <th>状态</th>
                        <th>创建时间</th>
                        <th>操作</th>
                    </tr>
                </thead>
                <tbody>
    `;
    
    groups.forEach(group => {
        const statusText = group.groupStatus === 0 ? '正常' : group.groupStatus === 1 ? '封禁' : '解散';
        const statusClass = group.groupStatus === 0 ? 'status-badge active' : group.groupStatus === 1 ? 'status-badge banned' : 'status-badge dissolved';
        const createTime = group.groupCreateTime ? formatTimestamp(group.groupCreateTime) : '-';
        
        tableHTML += `
            <tr>
                <td><code style="background: #e9ecef; padding: 4px 8px; border-radius: 4px;">${group.groupId || '-'}</code></td>
                <td>${group.groupName || '-'}</td>
                <td>${group.groupMemberNum || 0}</td>
                <td>${group.groupMaximum || '-'}</td>
                <td><span class="${statusClass}">${statusText}</span></td>
                <td>${createTime}</td>
                <td>
                    <button class="btn btn-info btn-sm" onclick="showGroupDetail('${group.groupId}')" title="查看详情">
                        <i class="fas fa-eye"></i>
                    </button>
                </td>
            </tr>
        `;
    });
    
    tableHTML += `
                </tbody>
            </table>
        </div>
    `;
    
    contentDiv.innerHTML = tableHTML;
}

/**
 * 清除查询结果
 */
function clearSearchResult() {
    const addressInput = document.getElementById('address-search-input');
    addressInput.value = '';
    
    const resultDiv = document.getElementById('address-search-result');
    resultDiv.style.display = 'none';
    
    const whitelistContent = document.getElementById('whitelist-search-content');
    const groupsContent = document.getElementById('groups-search-content');
    whitelistContent.innerHTML = '';
    groupsContent.innerHTML = '';
}
