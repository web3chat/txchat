/**
 * 每日活跃展示与查询
 */

function formatDateYMD(d) {
    const y = d.getFullYear();
    const m = String(d.getMonth() + 1).padStart(2, '0');
    const day = String(d.getDate()).padStart(2, '0');
    return `${y}-${m}-${day}`;
}

function defaultActivityRange(days) {
    const end = new Date();
    const start = new Date();
    start.setDate(end.getDate() - (days - 1));
    return { startDate: formatDateYMD(start), endDate: formatDateYMD(end) };
}

function ensureActivityDateInputs() {
    const startEl = document.getElementById('activity-start-date');
    const endEl = document.getElementById('activity-end-date');
    if (!startEl || !endEl) return;
    if (!startEl.value || !endEl.value) {
        const range = defaultActivityRange(30);
        startEl.value = range.startDate;
        endEl.value = range.endDate;
    }
}

async function loadDailyActiveHistory() {
    ensureActivityDateInputs();
    const startDate = document.getElementById('activity-start-date')?.value || '';
    const endDate = document.getElementById('activity-end-date')?.value || '';
    try {
        const resp = await apiClient.getDailyActiveList({
            startDate,
            endDate,
            page: 1,
            pageSize: 90
        });
        const list = (resp && resp.data && resp.data.list) ? resp.data.list : [];
        renderDailyActiveTable(list);
        drawDailyActiveChart('activity-chart', list);
    } catch (error) {
        console.error('加载每日活跃失败:', error);
        const tbody = document.getElementById('activity-table-body');
        if (tbody) {
            tbody.innerHTML = '<tr><td colspan="7" class="ca-metrics-empty">加载失败，请稍后重试</td></tr>';
        }
    }
}

async function loadDashboardDailyActive() {
    const range = defaultActivityRange(14);
    try {
        const resp = await apiClient.getDailyActiveList({
            startDate: range.startDate,
            endDate: range.endDate,
            page: 1,
            pageSize: 14
        });
        const list = (resp && resp.data && resp.data.list) ? resp.data.list : [];
        drawDailyActiveChart('dash-activity-chart', list);
    } catch (error) {
        console.error('加载仪表盘日活失败:', error);
    }
}

async function snapshotDailyActiveNow() {
    try {
        const resp = await apiClient.snapshotDailyActive();
        showNotification('今日活跃已快照保存', 'success');
        console.log('snapshot result', resp && resp.data);
        await loadDailyActiveHistory();
        await loadDashboardDailyActive();
    } catch (error) {
        console.error('快照失败:', error);
        showNotification('快照失败: ' + (error.message || '未知错误'), 'error');
    }
}

function renderDailyActiveTable(list) {
    const tbody = document.getElementById('activity-table-body');
    if (!tbody) return;
    if (!list || list.length === 0) {
        tbody.innerHTML = '<tr><td colspan="7" class="ca-metrics-empty">暂无日活记录，可点击「立即快照」</td></tr>';
        return;
    }
    const rows = list.map((item) => {
        const updated = item.updatedAt
            ? new Date(Number(item.updatedAt)).toLocaleString()
            : '-';
        return `<tr>
            <td>${item.statDate || '-'}</td>
            <td><button type="button" class="link-btn" onclick="openDailyActiveDetail('${item.statDate || ''}')">${formatNumber(item.activeUsers || 0)}</button></td>
            <td><button type="button" class="link-btn" onclick="openDailyActiveDetail('${item.statDate || ''}')">${formatNumber(item.activeGroups || 0)}</button></td>
            <td>${formatNumber(item.totalUsers || 0)}</td>
            <td>${formatNumber(item.totalGroups || 0)}</td>
            <td>${updated}</td>
            <td><button type="button" class="link-btn" onclick="openDailyActiveDetail('${item.statDate || ''}')">查看</button></td>
        </tr>`;
    }).join('');
    tbody.innerHTML = rows;
}

function drawDailyActiveChart(canvasId, list) {
    const canvas = document.getElementById(canvasId);
    if (!canvas || !canvas.getContext) return;
    const ctx = canvas.getContext('2d');
    const dpr = window.devicePixelRatio || 1;
    const cssW = canvas.clientWidth || 960;
    const cssH = canvas.clientHeight || 180;
    canvas.width = Math.floor(cssW * dpr);
    canvas.height = Math.floor(cssH * dpr);
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.clearRect(0, 0, cssW, cssH);

    const data = (list || []).slice().reverse();
    if (data.length === 0) {
        ctx.fillStyle = '#94a3b8';
        ctx.font = '13px Sora, sans-serif';
        ctx.fillText('暂无趋势数据', 16, cssH / 2);
        return;
    }

    const pad = { t: 18, r: 16, b: 28, l: 42 };
    const w = cssW - pad.l - pad.r;
    const h = cssH - pad.t - pad.b;
    const maxVal = Math.max(
        1,
        ...data.map((d) => Math.max(Number(d.activeUsers) || 0, Number(d.activeGroups) || 0))
    );

    ctx.strokeStyle = 'rgba(15,23,42,0.08)';
    ctx.lineWidth = 1;
    for (let i = 0; i <= 3; i++) {
        const y = pad.t + (h * i) / 3;
        ctx.beginPath();
        ctx.moveTo(pad.l, y);
        ctx.lineTo(pad.l + w, y);
        ctx.stroke();
    }

    const step = data.length === 1 ? w : w / (data.length - 1);
    const points = (key, color) => {
        ctx.beginPath();
        data.forEach((d, i) => {
            const x = pad.l + i * step;
            const y = pad.t + h - ((Number(d[key]) || 0) / maxVal) * h;
            if (i === 0) ctx.moveTo(x, y);
            else ctx.lineTo(x, y);
        });
        ctx.strokeStyle = color;
        ctx.lineWidth = 2.2;
        ctx.stroke();
        data.forEach((d, i) => {
            const x = pad.l + i * step;
            const y = pad.t + h - ((Number(d[key]) || 0) / maxVal) * h;
            ctx.beginPath();
            ctx.arc(x, y, 3, 0, Math.PI * 2);
            ctx.fillStyle = color;
            ctx.fill();
        });
    };

    points('activeUsers', '#0d9488');
    points('activeGroups', '#0f172a');

    ctx.fillStyle = '#64748b';
    ctx.font = '11px IBM Plex Mono, monospace';
    const labelEvery = Math.max(1, Math.ceil(data.length / 6));
    data.forEach((d, i) => {
        if (i % labelEvery !== 0 && i !== data.length - 1) return;
        const x = pad.l + i * step;
        const label = (d.statDate || '').slice(5);
        ctx.fillText(label, x - 12, cssH - 8);
    });

    ctx.font = '12px Sora, sans-serif';
    ctx.fillStyle = '#0d9488';
    ctx.fillText('日活用户', pad.l, 12);
    ctx.fillStyle = '#0f172a';
    ctx.fillText('日活群组', pad.l + 72, 12);
}

window.loadDailyActiveHistory = loadDailyActiveHistory;
window.loadDashboardDailyActive = loadDashboardDailyActive;
window.snapshotDailyActiveNow = snapshotDailyActiveNow;

const dailyActiveDetail = (() => {
    const pageSize = 50;
    let date = '';
    let addressPage = 1;
    let groupPage = 1;

    function escapeHtml(value) {
        return String(value == null ? '' : value)
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;');
    }

    function open(statDate) {
        date = statDate || '';
        addressPage = 1;
        groupPage = 1;
        const title = document.getElementById('daily-active-modal-title');
        if (title) title.textContent = (date || '今日') + ' 活跃明细';
        showModal('daily-active-modal');
        loadAddresses();
        loadGroups();
    }

    async function loadAddresses() {
        const listEl = document.getElementById('da-address-list');
        const totalEl = document.getElementById('da-address-total');
        const pager = document.getElementById('da-address-pager');
        if (listEl) listEl.innerHTML = '<li class="da-detail-empty">加载中…</li>';
        try {
            const resp = await apiClient.getDailyActiveAddresses({ date, page: addressPage, pageSize });
            const data = (resp && resp.data) || {};
            const list = data.list || [];
            const total = data.total || 0;
            if (date === '' && data.date && document.getElementById('daily-active-modal-title')) {
                date = data.date;
                document.getElementById('daily-active-modal-title').textContent = data.date + ' 活跃明细';
            }
            if (totalEl) totalEl.textContent = '共 ' + total + ' 个';
            if (!listEl) return;
            if (!list.length) {
                listEl.innerHTML = '<li class="da-detail-empty">这一天没有活跃地址</li>';
            } else {
                listEl.innerHTML = list.map((uid) => `<li>${escapeHtml(uid)}</li>`).join('');
            }
            if (pager) {
                const pages = Math.max(1, Math.ceil(total / (data.pageSize || pageSize)));
                pager.innerHTML = createPagination(data.page || addressPage, pages, 'dailyActiveDetail.goAddressPage');
            }
        } catch (error) {
            if (listEl) listEl.innerHTML = `<li class="da-detail-empty">${escapeHtml(error.message || '加载失败')}</li>`;
            if (pager) pager.innerHTML = '';
        }
    }

    async function loadGroups() {
        const listEl = document.getElementById('da-group-list');
        const totalEl = document.getElementById('da-group-total');
        const pager = document.getElementById('da-group-pager');
        if (listEl) listEl.innerHTML = '<li class="da-detail-empty">加载中…</li>';
        try {
            const resp = await apiClient.getDailyActiveGroups({ date, page: groupPage, pageSize });
            const data = (resp && resp.data) || {};
            const list = data.list || [];
            const total = data.total || 0;
            if (totalEl) totalEl.textContent = '共 ' + total + ' 个';
            if (!listEl) return;
            if (!list.length) {
                listEl.innerHTML = '<li class="da-detail-empty">这一天没有活跃群聊</li>';
            } else {
                listEl.innerHTML = list.map((item) => {
                    const name = item.groupName || '未命名';
                    return `<li><span class="da-name">${escapeHtml(name)}</span><span class="da-id">${escapeHtml(item.groupId || '')}</span></li>`;
                }).join('');
            }
            if (pager) {
                const pages = Math.max(1, Math.ceil(total / (data.pageSize || pageSize)));
                pager.innerHTML = createPagination(data.page || groupPage, pages, 'dailyActiveDetail.goGroupPage');
            }
        } catch (error) {
            if (listEl) listEl.innerHTML = `<li class="da-detail-empty">${escapeHtml(error.message || '加载失败')}</li>`;
            if (pager) pager.innerHTML = '';
        }
    }

    function goAddressPage(page) {
        addressPage = page;
        loadAddresses();
    }

    function goGroupPage(page) {
        groupPage = page;
        loadGroups();
    }

    return { open, goAddressPage, goGroupPage };
})();

function openDailyActiveDetail(date) {
    dailyActiveDetail.open(date);
}

window.openDailyActiveDetail = openDailyActiveDetail;
window.dailyActiveDetail = dailyActiveDetail;
