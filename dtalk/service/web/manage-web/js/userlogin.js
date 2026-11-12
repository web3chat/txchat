/**
 * 用户查询模块
 * 绑定信息与登录审计日志由后端聚合返回
 */

const userLoginModule = (() => {
    let currentPage = 1;
    let currentPageSize = 20;
    let currentAddress = '';
    let currentStartTime = 0;
    let currentEndTime = 0;

    function init() {
        const searchBtn = document.getElementById('ul-search-btn');
        const addressInput = document.getElementById('ul-address-input');
        const pageSizeSelect = document.getElementById('ul-page-size');

        if (searchBtn) {
            searchBtn.addEventListener('click', () => {
                currentPage = 1;
                doSearch();
            });
        }
        if (addressInput) {
            addressInput.addEventListener('keydown', (e) => {
                if (e.key === 'Enter') {
                    currentPage = 1;
                    doSearch();
                }
            });
        }
        if (pageSizeSelect) {
            pageSizeSelect.addEventListener('change', () => {
                currentPageSize = parseInt(pageSizeSelect.value) || 20;
                currentPage = 1;
                if (currentAddress) doSearch();
            });
        }
    }

    async function doSearch() {
        const addressInput = document.getElementById('ul-address-input');
        const startInput = document.getElementById('ul-start-time');
        const endInput = document.getElementById('ul-end-time');

        const address = (addressInput ? addressInput.value : '').trim();
        if (!address) {
            showNotification('请输入用户地址', 'warning');
            return;
        }

        currentAddress = address;
        currentStartTime = startInput && startInput.value ? new Date(startInput.value).getTime() : 0;
        currentEndTime = endInput && endInput.value ? new Date(endInput.value).getTime() : 0;

        try {
            const resp = await apiClient.getUserLoginHistory({
                address: currentAddress,
                page: currentPage,
                pageSize: currentPageSize,
                startTime: currentStartTime || undefined,
                endTime: currentEndTime || undefined,
            });

            if (resp && resp.code === 200) {
                renderResult(resp.data);
            } else {
                const message = (resp && resp.message) || '查询失败';
                showNotification(message, 'error');
                showQueryError(message);
            }
        } catch (e) {
            showQueryError((e && e.message) || '查询失败');
        }
    }

    function showQueryError(message) {
        const container = document.getElementById('ul-binding-card');
        if (container) {
            container.innerHTML = '<div class="ul-no-binding">绑定信息暂未取到</div>';
        }
        const tbody = document.getElementById('ul-log-tbody');
        if (tbody) {
            tbody.innerHTML = `<tr><td colspan="8" class="app-table-empty">${escapeHtml(message)}</td></tr>`;
        }
        const totalEl = document.getElementById('ul-total');
        if (totalEl) totalEl.textContent = '';
        const paginationEl = document.getElementById('ul-pagination');
        if (paginationEl) paginationEl.innerHTML = '';
    }

    function escapeHtml(value) {
        return String(value == null ? '' : value)
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;');
    }

    function renderResult(data) {
        renderBinding(data.binding);
        renderLogs(data.logs || [], data.total, data.page, data.pageSize);
    }

    function renderBinding(binding) {
        const container = document.getElementById('ul-binding-card');
        if (!container) return;

        if (!binding) {
            container.innerHTML = `<div class="ul-no-binding">该地址暂无账号绑定记录</div>`;
            return;
        }

        container.innerHTML = `
            <div class="ul-binding-grid">
                <div class="ul-binding-item">
                    <span class="ul-binding-label"><i class="fas fa-mobile-alt"></i> 手机号</span>
                    <span class="ul-binding-value">${binding.phone || '<span class="ul-empty">未绑定</span>'}</span>
                </div>
                <div class="ul-binding-item">
                    <span class="ul-binding-label"><i class="fab fa-weixin"></i> 微信号</span>
                    <span class="ul-binding-value">${binding.wechatId || '<span class="ul-empty">未绑定</span>'}</span>
                </div>
                <div class="ul-binding-item">
                    <span class="ul-binding-label"><i class="fas fa-heart"></i> 小红书</span>
                    <span class="ul-binding-value">${binding.xiaohongshuId || '<span class="ul-empty">未绑定</span>'}</span>
                </div>
                <div class="ul-binding-item">
                    <span class="ul-binding-label"><i class="fas fa-link"></i> 绑定时间</span>
                    <span class="ul-binding-value">${formatTimestamp(new Date(binding.createdAt).getTime())}</span>
                </div>
            </div>
        `;
    }

    function renderLogs(logs, total, page, pageSize) {
        const tbody = document.getElementById('ul-log-tbody');
        const totalEl = document.getElementById('ul-total');
        const paginationEl = document.getElementById('ul-pagination');

        if (totalEl) totalEl.textContent = `共 ${total} 条记录`;

        if (!tbody) return;

        if (!logs || logs.length === 0) {
            tbody.innerHTML = '<tr><td colspan="8" class="app-table-empty">暂无登录记录</td></tr>';
            if (paginationEl) paginationEl.innerHTML = '';
            return;
        }

        tbody.innerHTML = logs.map(log => `
            <tr>
                <td>${formatTimestamp(log.loginAt)}</td>
                <td><span class="ul-ip">${log.clientIp || '-'}</span></td>
                <td>${log.country || '-'}</td>
                <td>${log.cityRegion || '-'}</td>
                <td>${log.isp || '-'}</td>
                <td>${log.deviceType || '-'}</td>
                <td class="ul-device-name" title="${log.deviceName || ''}">${log.deviceName || '-'}</td>
                <td class="ul-uuid" title="${log.deviceUuid || ''}">${truncateUUID(log.deviceUuid)}</td>
            </tr>
        `).join('');

        if (paginationEl) {
            const totalPages = Math.ceil(total / pageSize);
            paginationEl.innerHTML = createPagination(page, totalPages, 'userLoginModule.goToPage');
        }
    }

    function goToPage(page) {
        currentPage = page;
        doSearch();
    }

    function truncateUUID(uuid) {
        if (!uuid) return '-';
        return uuid.length > 16 ? uuid.substring(0, 16) + '…' : uuid;
    }

    return { init, doSearch, goToPage };
})();

// 暴露到全局，供 HTML onclick 和 createPagination 使用
window.userLoginModule = userLoginModule;
