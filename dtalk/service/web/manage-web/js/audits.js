/**
 * 运营台操作留痕
 */

const auditsModule = (() => {
    let currentPage = 1;
    let currentPageSize = 20;

    const actionLabels = {
        login_ok: '登录成功',
        login_fail: '登录失败',
        logout: '退出登录',
        update_group_maximum: '修改群容量',
        update_group_status: '修改群状态',
        update_group_name: '修改群名称',
        add_whitelist: '添加白名单',
        update_whitelist: '修改白名单',
        delete_whitelist: '删除白名单',
        snapshot_daily_active: '日活快照'
    };

    const targetLabels = {
        admin: '管理员',
        group: '群组',
        whitelist: '白名单',
        metric: '指标'
    };

    function init() {
        document.getElementById('audit-search-btn')?.addEventListener('click', () => {
            currentPage = 1;
            doSearch();
        });
        document.getElementById('audit-page-size')?.addEventListener('change', () => {
            currentPageSize = parseInt(document.getElementById('audit-page-size').value, 10) || 20;
            currentPage = 1;
            doSearch();
        });
    }

    function selectedValues(id) {
        const text = document.getElementById(id)?.value || '';
        return text.split(/[\s,，]+/).map((s) => s.trim()).filter(Boolean);
    }

    async function doSearch() {
        const targetId = document.getElementById('audit-target-id')?.value.trim() || '';
        const operator = document.getElementById('audit-operator')?.value.trim() || '';
        try {
            const resp = await apiClient.getAudits({
                page: currentPage,
                pageSize: currentPageSize,
                action: selectedValues('audit-action'),
                targetType: selectedValues('audit-target-type'),
                targetId,
                operator
            });
            if (resp && resp.code === 200) {
                render(resp.data);
                return;
            }
            renderError((resp && resp.message) || '查询失败');
        } catch (e) {
            renderError((e && e.message) || '查询失败');
        }
    }

    function render(data) {
        const list = (data && data.list) || [];
        const total = (data && data.total) || 0;
        const page = (data && data.page) || 1;
        const pageSize = (data && data.pageSize) || currentPageSize;
        const tbody = document.getElementById('audit-tbody');
        const totalEl = document.getElementById('audit-total');
        const pager = document.getElementById('audit-pagination');
        if (totalEl) totalEl.textContent = `共 ${total} 条`;
        if (!tbody) return;
        if (!list.length) {
            tbody.innerHTML = '<tr><td colspan="6" class="app-table-empty">暂无操作记录</td></tr>';
            if (pager) pager.innerHTML = '';
            return;
        }
        tbody.innerHTML = list.map((item) => {
            const danger = item.action === 'delete_whitelist' || item.action === 'login_fail';
            return `<tr>
                <td>${formatTimestamp(item.createdAt)}</td>
                <td class="ll-uid-cell">${escapeHtml(item.operator || '-')}</td>
                <td><span class="status-badge ${danger ? 'status-banned' : 'status-active'}">${escapeHtml(actionLabels[item.action] || item.action || '-')}</span></td>
                <td>${escapeHtml(targetLabels[item.targetType] || item.targetType || '-')}${item.targetId ? ' ' + escapeHtml(item.targetId) : ''}</td>
                <td>${escapeHtml(item.extra || '-')}</td>
                <td>${escapeHtml(item.clientIp || '-')}</td>
            </tr>`;
        }).join('');
        if (pager) {
            const totalPages = Math.max(1, Math.ceil(total / pageSize));
            pager.innerHTML = createPagination(page, totalPages, 'auditsModule.goToPage');
        }
    }

    function renderError(message) {
        const tbody = document.getElementById('audit-tbody');
        if (tbody) {
            tbody.innerHTML = `<tr><td colspan="6" class="app-table-empty">${escapeHtml(message)}</td></tr>`;
        }
        const pager = document.getElementById('audit-pagination');
        if (pager) pager.innerHTML = '';
    }

    function escapeHtml(value) {
        return String(value == null ? '' : value)
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;');
    }

    function goToPage(page) {
        currentPage = page;
        doSearch();
    }

    function onSectionShown() {
        const ps = document.getElementById('audit-page-size');
        if (ps) currentPageSize = parseInt(ps.value, 10) || 20;
        doSearch();
    }

    return { init, goToPage, onSectionShown };
})();

window.auditsModule = auditsModule;
