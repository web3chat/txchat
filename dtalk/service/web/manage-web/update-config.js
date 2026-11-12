/**
 * 配置更新脚本
 * 按 config.js 的 API.BASE_URL 同步；页面在 :8000 时可自动推到 :18020
 */

function updateAPIConfig() {
    const currentHost = window.location.host;
    const currentProtocol = window.location.protocol;
    let apiPort = '18020';
    if (currentHost.includes(':8000')) {
        const hostPart = currentHost.replace(':8000', '');
        const apiBaseURL = `${currentProtocol}//${hostPart}:${apiPort}`;
        localStorage.setItem('apiBaseURL', apiBaseURL);
        localStorage.setItem('apiTimeout', '30000');
        console.log('API配置已更新:', {
            apiBaseURL: apiBaseURL,
            timeout: '30000'
        });
        if (window.apiClient) {
            window.apiClient.updateConfig(apiBaseURL, 30000);
            console.log('APIClient实例已更新');
        }
        return apiBaseURL;
    }
    console.warn('无法自动检测API配置，请手动设置');
    return null;
}

function applyConfigJsApiBase() {
    if (typeof CONFIG === 'undefined' || !CONFIG.API || !CONFIG.API.BASE_URL) {
        return null;
    }
    let base;
    try {
        base = String(CONFIG.API.BASE_URL).replace(/\/$/, '');
        new URL(base);
    } catch (e) {
        console.warn('config.js API.BASE_URL 无效:', CONFIG.API.BASE_URL, e);
        return null;
    }
    const timeout = parseInt(String(CONFIG.API.TIMEOUT), 10) || 30000;
    localStorage.setItem('apiBaseURL', base);
    localStorage.setItem('apiTimeout', String(timeout));
    if (window.apiClient) {
        window.apiClient.updateConfig(base, timeout);
    }
    console.log('已按 config.js 同步 API 地址:', base);
    return base;
}

// 优先用 config.js；避免 localStorage 残留错误地址
(function syncRemoteApiBaseBeforeApiClient() {
    if (typeof CONFIG === 'undefined' || !CONFIG.API) {
        return;
    }
    if (!CONFIG.API.BASE_URL) {
        if (typeof window !== 'undefined' && window.location && window.location.origin) {
            localStorage.setItem('apiBaseURL', window.location.origin);
        }
        return;
    }
    try {
        const base = String(CONFIG.API.BASE_URL).replace(/\/$/, '');
        new URL(base);
        const timeout = parseInt(String(CONFIG.API.TIMEOUT), 10) || 30000;
        localStorage.setItem('apiBaseURL', base);
        localStorage.setItem('apiTimeout', String(timeout));
    } catch (e) {
        console.warn('config.js 预同步 API 地址失败:', e);
    }
})();

document.addEventListener('DOMContentLoaded', function () {
    let apiURL = applyConfigJsApiBase();
    if (!apiURL) {
        apiURL = updateAPIConfig();
        if (apiURL) {
            console.log('自动配置完成，API地址:', apiURL);
        }
    }
});

function setAPIConfig(host, port = '18020') {
    const apiBaseURL = `http://${host}:${port}`;
    localStorage.setItem('apiBaseURL', apiBaseURL);
    localStorage.setItem('apiTimeout', '30000');
    if (window.apiClient) {
        window.apiClient.updateConfig(apiBaseURL, 30000);
    }
    console.log('手动配置完成，API地址:', apiBaseURL);
    return apiBaseURL;
}

function getCurrentConfig() {
    return {
        apiBaseURL: localStorage.getItem('apiBaseURL') || 'http://127.0.0.1:18020',
        timeout: parseInt(localStorage.getItem('apiTimeout')) || 30000
    };
}

function clearConfig() {
    localStorage.removeItem('apiBaseURL');
    localStorage.removeItem('apiTimeout');
    console.log('配置已清除，将使用默认配置');
}

window.updateAPIConfig = updateAPIConfig;
window.setAPIConfig = setAPIConfig;
window.getCurrentConfig = getCurrentConfig;
window.clearConfig = clearConfig;
