/**
 * 解析 JWT payload（不校验签名，仅读取 exp）
 */
function parseJwtPayload(token) {
    if (!token || typeof token !== 'string') return null;
    const parts = token.split('.');
    if (parts.length !== 3) return null;
    try {
        let base64 = parts[1].replace(/-/g, '+').replace(/_/g, '/');
        while (base64.length % 4) base64 += '=';
        return JSON.parse(atob(base64));
    } catch (e) {
        console.warn('[auth] JWT payload 解析失败', e);
        return null;
    }
}

/**
 * 用 JWT 标准字段 exp（秒）刷新 accessTokenExpiresAt。
 * 避免仅靠登录接口返回的 expiresIn 与本地存储不一致、或历史版本存错单位导致刷新页面被误判过期。
 */
function syncAccessTokenExpiryFromJWT() {
    const token = localStorage.getItem('accessToken');
    if (!token || token === 'null' || token === 'undefined') return;
    const payload = parseJwtPayload(token);
    if (!payload || typeof payload.exp !== 'number') return;
    localStorage.setItem('accessTokenExpiresAt', String(payload.exp * 1000));
}

// 简单的MetaMask登录流程
async function connectWallet() {
    if (!window.ethereum) {
        showNotification('未检测到 MetaMask，请先安装', 'warning');
        return;
    }
    
    try {
        // 1. 连接钱包
        let accounts;
        try {
            accounts = await window.ethereum.request({ method: 'eth_requestAccounts' });
        } catch (error) {
            console.error('连接钱包错误:', error);
            if (error.code === 4001) {
                showNotification('用户拒绝了连接钱包请求', 'warning');
            } else {
                const errorMsg = '连接钱包失败: ' + (error.message || '未知错误');
                console.error(errorMsg);
                showNotification(errorMsg, 'error');
                // 后备方案：如果通知不显示，使用 alert
                setTimeout(() => {
                    if (!document.querySelector('.notification.show')) {
                        alert('错误: ' + errorMsg);
                    }
                }, 100);
            }
            return;
        }
        
        const address = accounts[0];
        if (!address) {
            showNotification('未获取到钱包地址', 'error');
            showLoginErrorFlash('未获取到钱包地址');
            return;
        }
        
        // 2. 获取服务端nonce
        let nonceResp;
        let nonceData;
        try {
            nonceResp = await fetch(`${apiClient.baseURL}${apiClient.apiBase}/auth/nonce?address=${address}`);
            
            // 先读取响应文本
            const responseText = await nonceResp.text();
            console.log('Nonce响应:', { status: nonceResp.status, statusText: nonceResp.statusText, body: responseText });
            
            // 检查响应状态
            if (!nonceResp.ok) {
                let errorMsg = '获取登录凭证失败';
                try {
                    const errorJson = JSON.parse(responseText);
                    errorMsg = errorJson.message || errorMsg;
                    console.error('获取nonce失败，服务器返回:', errorJson);
                } catch (e) {
                    errorMsg = `服务器错误 (${nonceResp.status}): ${responseText.substring(0, 100)}`;
                    console.error('解析错误响应失败:', e, responseText);
                }
                showNotification(errorMsg, 'error');
                showLoginErrorFlash(errorMsg);
                return;
            }
            
            // 解析成功响应
            try {
                nonceData = JSON.parse(responseText);
            } catch (e) {
                console.error('解析nonce响应失败:', e, responseText);
                showNotification('服务器返回的登录凭证格式错误', 'error');
                showLoginErrorFlash('服务器返回的登录凭证格式错误');
                return;
            }
            
            if (!nonceData || !nonceData.data || !nonceData.data.nonce) {
                console.error('Nonce数据不完整:', nonceData);
                showNotification('服务器返回的登录凭证格式错误', 'error');
                showLoginErrorFlash('服务器返回的登录凭证格式错误');
                return;
            }
        } catch (error) {
            console.error('获取nonce过程发生异常:', error);
            let errorMsg = '获取登录凭证失败';
            if (error.name === 'TypeError' && error.message.includes('fetch')) {
                errorMsg = '网络连接失败，请检查网络或服务器地址';
            } else {
                errorMsg = '获取登录凭证失败: ' + (error.message || '未知错误');
            }
            showNotification(errorMsg, 'error');
            showLoginErrorFlash(errorMsg);
            return;
        }
        
        const nonce = nonceData.data.nonce;
        
        // 3. 用户签名
        let signature;
        try {
            signature = await window.ethereum.request({
                method: 'personal_sign',
                params: [nonce, address]
            });
        } catch (error) {
            if (error.code === 4001) {
                showNotification('用户拒绝了签名请求', 'warning');
            } else {
                const errorMsg = '签名失败: ' + (error.message || '未知错误');
                showNotification(errorMsg, 'error');
                showLoginErrorFlash(errorMsg);
            }
            return;
        }
        
        if (!signature) {
            showNotification('签名失败，未获取到签名数据', 'error');
            showLoginErrorFlash('签名失败，未获取到签名数据');
            return;
        }
        
        // 4. 发送到后端验证
        let verifyResp;
        let verifyData;
        try {
            verifyResp = await fetch(`${apiClient.baseURL}${apiClient.apiBase}/auth/verify`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ address, signature })
            });
            
            // 先读取响应文本
            const responseText = await verifyResp.text();
            console.log('验证响应:', { status: verifyResp.status, statusText: verifyResp.statusText, body: responseText });
            
            // 检查响应状态
            if (!verifyResp.ok) {
                let errorMsg = '登录验证失败';
                try {
                    const errorJson = JSON.parse(responseText);
                    errorMsg = errorJson.message || errorMsg;
                    console.error('验证失败，服务器返回:', errorJson);
                } catch (e) {
                    errorMsg = `验证失败 (${verifyResp.status}): ${responseText.substring(0, 100)}`;
                    console.error('解析错误响应失败:', e, responseText);
                }
                
                // 显示错误通知
                showNotification(errorMsg, 'error');
                console.error('登录验证失败，已显示通知:', errorMsg);
                
                // 显示登录页面红色闪烁提示
                showLoginErrorFlash(errorMsg);
                
                return;
            }
            
            // 解析成功响应
            try {
                verifyData = JSON.parse(responseText);
            } catch (e) {
                console.error('解析成功响应失败:', e, responseText);
                showNotification('服务器返回的登录令牌格式错误', 'error');
                showLoginErrorFlash('服务器返回的登录令牌格式错误');
                return;
            }
            
            if (!verifyData || !verifyData.data || !verifyData.data.accessToken) {
                console.error('登录令牌数据不完整:', verifyData);
                showNotification('服务器返回的登录令牌格式错误', 'error');
                showLoginErrorFlash('服务器返回的登录令牌格式错误');
                return;
            }
        } catch (error) {
            console.error('验证过程发生异常:', error);
            let errorMsg = '登录验证失败';
            if (error.name === 'TypeError' && error.message.includes('fetch')) {
                errorMsg = '网络连接失败，无法验证登录';
            } else {
                errorMsg = '登录验证失败: ' + (error.message || '未知错误');
            }
            showNotification(errorMsg, 'error');
            showLoginErrorFlash(errorMsg);
            return;
        }
        
        // 5. 保存登录信息
        localStorage.setItem('accessToken', verifyData.data.accessToken);
        if (verifyData.data.expiresIn) {
            // 服务端返回 Unix 秒时间戳（绝对过期时刻）
            localStorage.setItem('accessTokenExpiresAt', String(verifyData.data.expiresIn * 1000));
        }
        syncAccessTokenExpiryFromJWT();
        localStorage.setItem('adminAddress', address);
        renderAdminStatus();
        toggleLoginOverlay();
        scheduleAutoLogout();
        
        // 6. 登录后先加载配置，再初始化应用
        try {
            // 先加载服务配置信息（需要鉴权）
            if (typeof loadServiceConfig === 'function') {
                await loadServiceConfig();
            }
            
            // 初始化导航
            if (typeof setupNavigation === 'function') {
                setupNavigation();
            }
            
            // 显示仪表盘并加载数据
            if (typeof showSection === 'function') {
                showSection('dashboard');
            }
            
            // 加载仪表盘数据
            if (typeof loadDashboardData === 'function') {
                loadDashboardData();
            }
        } catch (e) {
            console.error('登录后初始化失败:', e);
            // 即使初始化失败，也不影响登录成功提示
        }
        
        showNotification('登录成功', 'success');
    } catch (e) {
        console.error('登录过程发生未知错误:', e);
        const errorMsg = '登录失败: ' + (e.message || '未知错误');
        showNotification(errorMsg, 'error');
        showLoginErrorFlash(errorMsg);
    }
}

function logoutAdmin() {
    const token = localStorage.getItem('accessToken');
    const finish = () => {
        localStorage.removeItem('accessToken');
        localStorage.removeItem('accessTokenExpiresAt');
        localStorage.removeItem('adminAddress');
        renderAdminStatus();
        toggleLoginOverlay();
    };
    if (!token || token === 'null' || token === 'undefined' || typeof apiClient === 'undefined') {
        finish();
        return;
    }
    apiClient.logout().catch(() => {}).finally(finish);
}

function renderAdminStatus() {
    const area = document.getElementById('admin-area');
    if (!area) return;
    const token = localStorage.getItem('accessToken');
    const addr = localStorage.getItem('adminAddress');
    area.className = 'user-chip';
    if (token && addr) {
        const short = addr.slice(0, 6) + '...' + addr.slice(-4);
        const mark = (addr.replace(/^0x/i, '').charAt(0) || 'A').toUpperCase();
        area.innerHTML = `<span class="avatar">${mark}</span>
            <span class="user-meta">
                <strong class="admin-address">${short}</strong>
                <small>钱包已连接</small>
            </span>`;
    } else {
        area.innerHTML = `<span class="avatar">—</span>
            <span class="user-meta"><strong>未登录</strong><small>请连接钱包</small></span>`;
    }
}

document.addEventListener('DOMContentLoaded', function() {
    syncAccessTokenExpiryFromJWT();

    renderAdminStatus();

    // 检查 token 是否过期（毫秒时间戳，与 JWT exp 对齐）
    const exp = parseInt(localStorage.getItem('accessTokenExpiresAt') || '0', 10);
    if (exp && Date.now() > exp) {
        logoutAdmin();
    } else {
        scheduleAutoLogout();
    }

    // 切换登录页面和业务页面显示
    toggleLoginOverlay();
});

function isLoggedIn() {
    const token = localStorage.getItem('accessToken');
    return !!token;
}

function requireLogin() {
    if (!isLoggedIn()) {
        showNotification('请先连接钱包登录后再查看', 'warning');
        return false;
    }
    return true;
}

window.isLoggedIn = isLoggedIn;
window.requireLogin = requireLogin;
window.toggleLoginOverlay = toggleLoginOverlay;

function toggleLoginOverlay() {
    const loginPage = document.getElementById('login-page');
    const appContainer = document.getElementById('app-container');
    const loggedIn = isLoggedIn();
    
    // 未登录时：显示登录页面，隐藏业务页面
    if (loginPage) {
        // 与 CSS 中 .login-page { display: grid } 保持一致
        loginPage.style.display = loggedIn ? 'none' : 'grid';
    }
    
    // 登录后：显示业务页面。必须用 grid，不能用 block（否则侧栏占满屏、主内容被挤到下方变成“空白页”）
    if (appContainer) {
        appContainer.style.display = loggedIn ? 'grid' : 'none';
    }
    
    // 如果登录成功，初始化导航
    if (loggedIn && appContainer) {
        // 确保导航功能正常工作
        if (typeof setupNavigation === 'function') {
            setupNavigation();
        }
        if (typeof startSecurityWatch === 'function') {
            startSecurityWatch();
        }
    } else if (typeof stopSecurityWatch === 'function') {
        stopSecurityWatch();
    }
}

let _autoLogoutTimer = null;
function scheduleAutoLogout() {
    if (_autoLogoutTimer) {
        clearTimeout(_autoLogoutTimer);
        _autoLogoutTimer = null;
    }
    syncAccessTokenExpiryFromJWT();
    const exp = parseInt(localStorage.getItem('accessTokenExpiresAt') || '0', 10);
    if (!exp) return;
    const ms = exp - Date.now() - 3000; // 提前3秒退出
    if (ms > 0) {
        _autoLogoutTimer = setTimeout(() => {
            showNotification('登录已过期，请重新登录', 'warning');
            logoutAdmin();
        }, ms);
    }
}

/**
 * 显示登录失败红色闪烁提示
 */
function showLoginErrorFlash(errorMessage) {
    const loginPage = document.getElementById('login-page');
    const errorOverlay = document.getElementById('login-error-overlay');
    
    if (!loginPage) {
        console.error('登录页面元素不存在');
        alert('登录失败: ' + errorMessage);
        return;
    }
    
    // 添加错误闪烁类
    loginPage.classList.add('error-flash');
    
    // 显示错误遮罩层
    if (errorOverlay) {
        errorOverlay.classList.add('show');
    }
    
    // 在登录区域显示错误消息
    const loginCard = loginPage.querySelector('.login-card') || loginPage.querySelector('.login-stage');
    if (loginCard) {
        // 移除之前的错误消息
        const existingError = loginCard.querySelector('.login-error-message');
        if (existingError) {
            existingError.remove();
        }
        
        // 创建错误消息元素
        const errorMsgEl = document.createElement('div');
        errorMsgEl.className = 'login-error-message';
        errorMsgEl.innerHTML = `
            <div class="login-error-content">
                <i class="fas fa-exclamation-circle"></i>
                <span>${errorMessage}</span>
            </div>
        `;
        
        const loginBody = loginCard.querySelector('.login-body');
        if (loginBody) {
            loginBody.insertBefore(errorMsgEl, loginBody.firstChild);
        } else {
            const hint = loginCard.querySelector('.login-hint');
            if (hint) {
                loginCard.insertBefore(errorMsgEl, hint);
            } else {
                loginCard.appendChild(errorMsgEl);
            }
        }
    }
    
    // 1.5秒后移除闪烁效果
    setTimeout(() => {
        loginPage.classList.remove('error-flash');
        if (errorOverlay) {
            errorOverlay.classList.remove('show');
        }
    }, 1500);
    
    // 3秒后移除错误消息
    setTimeout(() => {
        const errorMsgEl = loginCard?.querySelector('.login-error-message');
        if (errorMsgEl) {
            errorMsgEl.style.opacity = '0';
            errorMsgEl.style.transition = 'opacity 0.3s ease';
            setTimeout(() => {
                errorMsgEl.remove();
            }, 300);
        }
    }, 3000);
}


