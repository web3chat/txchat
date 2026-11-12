/**
 * 防攻击提醒：拉取近期被拉黑的探测 IP，显示在页面消息区。
 */
const securityWatch = {
    timer: null,
    seen: new Set()
};

function startSecurityWatch() {
    if (securityWatch.timer) {
        return;
    }
    pollSecurityAlerts();
    securityWatch.timer = setInterval(pollSecurityAlerts, 20000);
}

function stopSecurityWatch() {
    if (securityWatch.timer) {
        clearInterval(securityWatch.timer);
        securityWatch.timer = null;
    }
    const feed = document.getElementById('security-feed');
    if (feed) {
        feed.hidden = true;
        feed.innerHTML = '';
    }
}

async function pollSecurityAlerts() {
    if (typeof isLoggedIn !== 'function' || !isLoggedIn() || typeof apiClient === 'undefined') {
        return;
    }
    const token = localStorage.getItem('accessToken');
    const base = String(apiClient.baseURL || '').replace(/\/$/, '');
    try {
        const response = await fetch(base + '/api/v1/groupmanage/security/alerts', {
            headers: { Authorization: 'Bearer ' + token }
        });
        if (!response.ok) {
            return;
        }
        const payload = await response.json();
        const alerts = (payload.data && payload.data.alerts) || [];
        renderSecurityFeed(alerts);
    } catch (error) {
        console.error('拉取防攻击提醒失败:', error);
    }
}

function renderSecurityFeed(alerts) {
    const feed = document.getElementById('security-feed');
    if (!feed) {
        return;
    }
    const active = (alerts || []).filter(function (item) {
        return item && item.remainSec > 0;
    });
    if (active.length === 0) {
        feed.hidden = true;
        feed.innerHTML = '';
        return;
    }
    feed.hidden = false;
    feed.innerHTML = active.map(function (item) {
        const remain = formatBanRemain(item.remainSec);
        return '<p>' + escapeSecurityText(item.message) + '，剩余 ' + remain + '</p>';
    }).join('');
    active.forEach(function (item) {
        if (securityWatch.seen.has(item.id)) {
            return;
        }
        securityWatch.seen.add(item.id);
        if (typeof showNotification === 'function') {
            showNotification(item.message, 'warning');
        }
    });
}

function formatBanRemain(sec) {
    const left = Math.max(1, Number(sec) || 0);
    const min = Math.ceil(left / 60);
    return min + ' 分钟';
}

function escapeSecurityText(text) {
    return String(text || '')
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;');
}

window.startSecurityWatch = startSecurityWatch;
window.stopSecurityWatch = stopSecurityWatch;
