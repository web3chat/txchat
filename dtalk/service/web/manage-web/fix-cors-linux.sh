#!/bin/bash

# CORS修复脚本 - Linux部署版本
# 用于修复跨域访问问题

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 获取当前服务器IP
get_server_ip() {
    # 尝试多种方法获取公网IP
    local ip=""
    
    # 方法1: 使用curl获取公网IP
    if command -v curl &> /dev/null; then
        ip=$(curl -s ifconfig.me 2>/dev/null || curl -s ipinfo.io/ip 2>/dev/null || curl -s icanhazip.com 2>/dev/null)
    fi
    
    # 方法2: 使用wget
    if [ -z "$ip" ] && command -v wget &> /dev/null; then
        ip=$(wget -qO- ifconfig.me 2>/dev/null || wget -qO- ipinfo.io/ip 2>/dev/null)
    fi
    
    # 方法3: 使用hostname
    if [ -z "$ip" ]; then
        ip=$(hostname -I | awk '{print $1}')
    fi
    
    echo "$ip"
}

# 更新前端配置
update_frontend_config() {
    local server_ip="$1"
    
    log_info "更新前端配置..."
    
    # 创建配置更新脚本
    cat > update-config.js << 'EOF'
/**
 * 配置更新脚本
 * 用于更新前端API配置以适配不同的部署环境
 */

// 配置更新函数
function updateAPIConfig() {
    // 获取当前域名和端口
    const currentHost = window.location.host;
    const currentProtocol = window.location.protocol;
    
    // 构建API基础URL
    // 如果当前是8000端口，则API端口是18020
    let apiPort = '18020';
    if (currentHost.includes(':8000')) {
        // 从当前地址提取主机部分，替换端口
        const hostPart = currentHost.replace(':8000', '');
        const apiBaseURL = `${currentProtocol}//${hostPart}:${apiPort}`;
        
        // 更新localStorage中的配置
        localStorage.setItem('apiBaseURL', apiBaseURL);
        localStorage.setItem('apiTimeout', '30000');
        
        console.log('API配置已更新:', {
            apiBaseURL: apiBaseURL,
            timeout: '30000'
        });
        
        // 如果APIClient已存在，更新其实例
        if (window.apiClient) {
            window.apiClient.updateConfig(apiBaseURL, 30000);
            console.log('APIClient实例已更新');
        }
        
        return apiBaseURL;
    } else {
        console.warn('无法自动检测API配置，请手动设置');
        return null;
    }
}

// 页面加载时自动更新配置
document.addEventListener('DOMContentLoaded', function() {
    const apiURL = updateAPIConfig();
    if (apiURL) {
        console.log('自动配置完成，API地址:', apiURL);
    }
});

// 手动配置函数
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

// 获取当前配置
function getCurrentConfig() {
    return {
        apiBaseURL: localStorage.getItem('apiBaseURL') || 'http://localhost:18020',
        timeout: parseInt(localStorage.getItem('apiTimeout')) || 30000
    };
}

// 清除配置（恢复默认）
function clearConfig() {
    localStorage.removeItem('apiBaseURL');
    localStorage.removeItem('apiTimeout');
    console.log('配置已清除，将使用默认配置');
}

// 导出到全局作用域
window.updateAPIConfig = updateAPIConfig;
window.setAPIConfig = setAPIConfig;
window.getCurrentConfig = getCurrentConfig;
window.clearConfig = clearConfig;
EOF

    # 更新HTML文件，添加配置脚本
    if [ -f "index.html" ]; then
        # 备份原文件
        cp index.html index.html.backup
        
        # 在JavaScript部分添加配置脚本
        sed -i '/<!-- JavaScript -->/a\    <script src="update-config.js?v=20251029"></script>' index.html
        
        log_success "前端配置已更新"
    else
        log_error "未找到 index.html 文件"
        return 1
    fi
}

# 显示后端配置更新说明
show_backend_config() {
    local server_ip="$1"
    
    log_info "后端配置更新说明:"
    echo ""
    echo "需要在后端服务器的 groupmanage 服务中更新 CORS 配置："
    echo ""
    echo "文件: service/groupmanage/server/http/http.go"
    echo "位置: 第57行的 AllowOrigins 配置"
    echo ""
    echo "将:"
    echo "    AllowOrigins: []string{\"http://localhost:8000\"},"
    echo ""
    echo "修改为:"
    echo "    AllowOrigins: []string{\"http://localhost:8000\", \"http://${server_ip}:8000\"},"
    echo ""
    echo "然后重新编译和启动 groupmanage 服务。"
    echo ""
}

# 测试连接
test_connection() {
    local server_ip="$1"
    local api_url="http://${server_ip}:18020"
    
    log_info "测试API连接..."
    
    if command -v curl &> /dev/null; then
        if curl -s --connect-timeout 5 "${api_url}/api/v1/groupmanage/auth/nonce?address=0x123" > /dev/null 2>&1; then
            log_success "API服务连接正常"
        else
            log_warning "API服务连接失败，请检查后端服务是否启动"
        fi
    else
        log_warning "curl 未安装，无法测试连接"
    fi
}

# 主函数
main() {
    log_info "开始修复CORS跨域问题..."
    
    # 获取服务器IP
    local server_ip=$(get_server_ip)
    if [ -z "$server_ip" ]; then
        log_error "无法获取服务器IP地址"
        exit 1
    fi
    
    log_info "检测到服务器IP: $server_ip"
    
    # 更新前端配置
    update_frontend_config "$server_ip"
    
    # 显示后端配置说明
    show_backend_config "$server_ip"
    
    # 测试连接
    test_connection "$server_ip"
    
    log_success "前端配置修复完成！"
    log_info "请按照上述说明更新后端CORS配置，然后重启服务。"
    log_info "修复完成后，访问: http://${server_ip}:8000"
}

# 执行主函数
main "$@"
