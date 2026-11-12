/**
 * 全站用户登录日志列表
 */

const loginLogsAllModule = (() => {
    let currentPage = 1;
    let currentPageSize = 20;

    function parseUidFilters(text) {
        if (!text || !String(text).trim()) return [];
        return String(text)
            .split(/[\s,，]+/)
            .map((s) => s.trim())
            .filter(Boolean);
    }

    function escapeHtml(s) {
        if (s == null || s === '') return '';
        return String(s)
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;');
    }

    function displayUid(uid) {
        if (!uid) return '-';
        return escapeHtml(uid);
    }

    function truncateUUID(uuid) {
        if (!uuid) return '-';
        return uuid.length > 16 ? uuid.substring(0, 16) + '…' : uuid;
    }

    function init() {
        const ps = document.getElementById('ll-page-size');
        if (ps) currentPageSize = parseInt(ps.value, 10) || 20;

        document.getElementById('ll-search-btn')?.addEventListener('click', () => {
            currentPage = 1;
            doSearch();
        });

        document.getElementById('ll-page-size')?.addEventListener('change', () => {
            currentPageSize = parseInt(document.getElementById('ll-page-size').value, 10) || 20;
            currentPage = 1;
            doSearch();
        });
    }

    async function doSearch() {
        const startInput = document.getElementById('ll-start-time');
        const endInput = document.getElementById('ll-end-time');
        const uidInput = document.getElementById('ll-uid-filter');

        const startTime = startInput && startInput.value ? new Date(startInput.value).getTime() : 0;
        const endTime = endInput && endInput.value ? new Date(endInput.value).getTime() : 0;
        const uids = parseUidFilters(uidInput ? uidInput.value : '');

        try {
            const resp = await apiClient.getAllLoginLogs({
                page: currentPage,
                pageSize: currentPageSize,
                startTime: startTime || undefined,
                endTime: endTime || undefined,
                uid: uids.length ? uids : undefined,
            });

            if (resp && resp.code === 200) {
                renderTable(resp.data);
            } else {
                const msg = (resp && resp.message) || '查询失败';
                showNotification(msg, 'error');
                console.warn('[loginlogs] 非成功响应', resp);
            }
        } catch (e) {
            console.error('[loginlogs] 查询异常（若上方已有 toast，可忽略本条）', e);
        }
    }

    function renderTable(data) {
        const list = (data && data.list) || [];
        const total = (data && data.total) || 0;
        const page = (data && data.page) || 1;
        const pageSize = (data && data.pageSize) || currentPageSize;

        const tbody = document.getElementById('ll-log-tbody');
        const totalEl = document.getElementById('ll-total');
        const paginationEl = document.getElementById('ll-pagination');

        if (totalEl) totalEl.textContent = `共 ${total} 条记录`;

        if (!tbody) return;

        if (!list.length) {
            tbody.innerHTML =
                '<tr><td colspan="9" class="app-table-empty">暂无登录记录</td></tr>';
            if (paginationEl) paginationEl.innerHTML = '';
            return;
        }

        tbody.innerHTML = list
            .map(
                (log) => `
            <tr>
                <td class="ll-uid-cell">${displayUid(log.uid)}</td>
                <td>${formatTimestamp(log.loginAt)}</td>
                <td><span class="ul-ip">${log.clientIp || '-'}</span></td>
                <td>${log.country || '-'}</td>
                <td>${log.cityRegion || '-'}</td>
                <td>${log.isp || '-'}</td>
                <td>${log.deviceType || '-'}</td>
                <td class="ul-device-name" title="${log.deviceName || ''}">${log.deviceName || '-'}</td>
                <td class="ul-uuid" title="${log.deviceUuid || ''}">${truncateUUID(log.deviceUuid)}</td>
            </tr>`
            )
            .join('');

        if (paginationEl) {
            const totalPages = Math.max(1, Math.ceil(total / pageSize));
            paginationEl.innerHTML = createPagination(page, totalPages, 'loginLogsAllModule.goToPage');
        }
    }

    function goToPage(page) {
        currentPage = page;
        doSearch();
    }

    function onSectionShown() {
        const ps = document.getElementById('ll-page-size');
        if (ps) currentPageSize = parseInt(ps.value, 10) || 20;
        currentPage = 1;
        doSearch();
    }

    return { init, goToPage, onSectionShown };
})();

window.loginLogsAllModule = loginLogsAllModule;
