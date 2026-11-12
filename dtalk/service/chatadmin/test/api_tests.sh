#!/bin/bash

# 群组管理API测试脚本
# 使用方法: ./api_tests.sh [选项]
# 选项:
#   -h, --help     显示帮助信息
#   -v, --verbose  详细输出模式
#   -q, --quiet    静默模式
#   -t, --timeout  设置超时时间（秒，默认30）
#   -u, --url      设置服务URL（默认http://localhost:18020）

# 默认配置
BASE_URL="http://localhost:18020"
API_BASE="/api/v1/groupmanage"
VERBOSE=false
QUIET=false
TIMEOUT=30
TEST_RESULTS=()
PASSED_TESTS=0
FAILED_TESTS=0

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 帮助信息
show_help() {
    echo "群组管理API测试脚本"
    echo ""
    echo "使用方法: $0 [选项]"
    echo ""
    echo "选项:"
    echo "  -h, --help     显示帮助信息"
    echo "  -v, --verbose  详细输出模式"
    echo "  -q, --quiet    静默模式"
    echo "  -t, --timeout  设置超时时间（秒，默认30）"
    echo "  -u, --url      设置服务URL（默认http://localhost:18020）"
    echo ""
    echo "示例:"
    echo "  $0                    # 使用默认配置运行测试"
    echo "  $0 -v                 # 详细模式"
    echo "  $0 -u http://192.168.1.100:18020  # 指定服务地址"
    echo "  $0 -t 60              # 设置60秒超时"
}

# 解析命令行参数
while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_help
            exit 0
            ;;
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        -q|--quiet)
            QUIET=true
            shift
            ;;
        -t|--timeout)
            TIMEOUT="$2"
            shift 2
            ;;
        -u|--url)
            BASE_URL="$2"
            shift 2
            ;;
        *)
            echo "未知选项: $1"
            show_help
            exit 1
            ;;
    esac
done

# 日志函数
log_info() {
    if [ "$QUIET" = false ]; then
        echo -e "${BLUE}[INFO]${NC} $1"
    fi
}

log_success() {
    if [ "$QUIET" = false ]; then
        echo -e "${GREEN}[SUCCESS]${NC} $1"
    fi
}

log_warning() {
    if [ "$QUIET" = false ]; then
        echo -e "${YELLOW}[WARNING]${NC} $1"
    fi
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_verbose() {
    if [ "$VERBOSE" = true ] && [ "$QUIET" = false ]; then
        echo -e "${BLUE}[VERBOSE]${NC} $1"
    fi
}

# 测试结果记录
record_test() {
    local test_name="$1"
    local status="$2"
    local message="$3"
    
    TEST_RESULTS+=("$test_name:$status:$message")
    
    if [ "$status" = "PASS" ]; then
        ((PASSED_TESTS++))
        log_success "$test_name: $message"
    else
        ((FAILED_TESTS++))
        log_error "$test_name: $message"
    fi
}

# 检查依赖
check_dependencies() {
    log_info "检查依赖..."
    
    if ! command -v curl &> /dev/null; then
        log_error "curl 未安装，请先安装 curl"
        exit 1
    fi
    
    if ! command -v jq &> /dev/null; then
        log_warning "jq 未安装，JSON输出将不会被格式化"
        JQ_AVAILABLE=false
    else
        JQ_AVAILABLE=true
    fi
    
    log_success "依赖检查完成"
}

# 测试服务连接
test_service_connection() {
    log_info "测试服务连接..."
    
    local response_code
    response_code=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout "$TIMEOUT" "$BASE_URL$API_BASE/groups/list" 2>/dev/null)
    
    if [ "$response_code" = "200" ]; then
        record_test "服务连接" "PASS" "服务连接正常 (HTTP $response_code)"
        return 0
    else
        record_test "服务连接" "FAIL" "服务连接失败 (HTTP $response_code)"
        log_error "请确保服务正在运行在 $BASE_URL"
        return 1
    fi
}

# 执行API请求并记录结果
api_request() {
    local method="$1"
    local url="$2"
    local data="$3"
    local expected_code="$4"
    local test_name="$5"
    
    log_verbose "执行请求: $method $url"
    if [ -n "$data" ]; then
        log_verbose "请求数据: $data"
    fi
    
    local response
    local http_code
    
    if [ -n "$data" ]; then
        response=$(curl -s -w "\n%{http_code}" -X "$method" \
            -H "Content-Type: application/json" \
            -d "$data" \
            --connect-timeout "$TIMEOUT" \
            "$BASE_URL$url" 2>/dev/null)
    else
        response=$(curl -s -w "\n%{http_code}" -X "$method" \
            --connect-timeout "$TIMEOUT" \
            "$BASE_URL$url" 2>/dev/null)
    fi
    
    http_code=$(echo "$response" | tail -n1)
    response_body=$(echo "$response" | head -n -1)
    
    if [ "$http_code" = "$expected_code" ]; then
        record_test "$test_name" "PASS" "HTTP $http_code"
        if [ "$VERBOSE" = true ] && [ "$JQ_AVAILABLE" = true ]; then
            echo "$response_body" | jq . 2>/dev/null || echo "$response_body"
        elif [ "$VERBOSE" = true ]; then
            echo "$response_body"
        fi
        return 0
    else
        record_test "$test_name" "FAIL" "期望 HTTP $expected_code，实际 HTTP $http_code"
        if [ "$VERBOSE" = true ]; then
            echo "响应内容: $response_body"
        fi
        return 1
    fi
}

# 获取JSON字段值
get_json_field() {
    local json="$1"
    local field="$2"
    
    if [ "$JQ_AVAILABLE" = true ]; then
        echo "$json" | jq -r "$field" 2>/dev/null
    else
        # 简单的grep实现（不完美，但可用）
        echo "$json" | grep -o "\"$field\"[[:space:]]*:[[:space:]]*\"[^\"]*\"" | sed 's/.*"\([^"]*\)".*/\1/'
    fi
}

echo "=========================================="
echo "群组管理API测试脚本"
echo "=========================================="
echo "服务地址: $BASE_URL"
echo "超时设置: ${TIMEOUT}秒"
echo "详细模式: $VERBOSE"
echo "=========================================="

# 检查依赖
check_dependencies

# 测试服务连接
if ! test_service_connection; then
    exit 1
fi

# 群组管理API测试
log_info "开始群组管理API测试..."

# 1. 获取群组列表
log_info "测试获取群组列表..."
GROUP_LIST_RESPONSE=$(curl -s -X GET --connect-timeout "$TIMEOUT" "$BASE_URL$API_BASE/groups/list" 2>/dev/null)
if api_request "GET" "$API_BASE/groups/list" "" "200" "获取群组列表"; then
    # 提取群组ID用于后续测试
    GROUP_ID=$(get_json_field "$GROUP_LIST_RESPONSE" ".data.list[0].groupId")
    if [ "$GROUP_ID" != "null" ] && [ "$GROUP_ID" != "" ]; then
        log_verbose "找到群组ID: $GROUP_ID"
    else
        log_warning "未找到群组ID，部分测试将被跳过"
    fi
fi

# 2. 获取群组统计信息
log_info "测试获取群组统计信息..."
api_request "GET" "$API_BASE/groups/stats" "" "200" "获取群组统计信息"

# 3. 获取特定群组信息
log_info "测试获取特定群组信息..."
if [ "$GROUP_ID" != "null" ] && [ "$GROUP_ID" != "" ]; then
    api_request "GET" "$API_BASE/groups/info/$GROUP_ID" "" "200" "获取特定群组信息"
else
    record_test "获取特定群组信息" "SKIP" "未找到群组ID"
fi

# 4. 更新群组最大人数
log_info "测试更新群组最大人数..."
if [ "$GROUP_ID" != "null" ] && [ "$GROUP_ID" != "" ]; then
    api_request "PUT" "$API_BASE/groups/maximum/$GROUP_ID" '{"groupMaximum": 500}' "200" "更新群组最大人数"
else
    record_test "更新群组最大人数" "SKIP" "未找到群组ID"
fi

# 5. 更新群组状态
log_info "测试更新群组状态..."
if [ "$GROUP_ID" != "null" ] && [ "$GROUP_ID" != "" ]; then
    api_request "PUT" "$API_BASE/groups/status/$GROUP_ID" '{"groupStatus": 0}' "200" "更新群组状态"
else
    record_test "更新群组状态" "SKIP" "未找到群组ID"
fi

# 6. 更新群组名称
log_info "测试更新群组名称..."
if [ "$GROUP_ID" != "null" ] && [ "$GROUP_ID" != "" ]; then
    api_request "PUT" "$API_BASE/groups/name/$GROUP_ID" '{"groupName": "测试群组名称"}' "200" "更新群组名称"
else
    record_test "更新群组名称" "SKIP" "未找到群组ID"
fi

# 7. 根据状态查询群组
log_info "测试根据状态查询群组..."
api_request "GET" "$API_BASE/groups/by-status/0" "" "200" "查询正常状态群组"
api_request "GET" "$API_BASE/groups/by-status/1" "" "200" "查询封禁状态群组"
api_request "GET" "$API_BASE/groups/by-status/2" "" "200" "查询解散状态群组"

# 8. 测试分页查询
log_info "测试分页查询..."
api_request "GET" "$API_BASE/groups/list?page=1&pageSize=2" "" "200" "分页查询群组列表"

# 9. 测试状态筛选
log_info "测试状态筛选..."
api_request "GET" "$API_BASE/groups/list?status=0" "" "200" "按状态筛选群组"

# 白名单管理API测试
log_info "开始白名单管理API测试..."

# 1. 获取白名单列表
log_info "测试获取白名单列表..."
WHITELIST_LIST_RESPONSE=$(curl -s -X GET --connect-timeout "$TIMEOUT" "$BASE_URL$API_BASE/whitelist/list" 2>/dev/null)
if api_request "GET" "$API_BASE/whitelist/list" "" "200" "获取白名单列表"; then
    # 提取白名单ID用于后续测试
    WHITELIST_ID=$(get_json_field "$WHITELIST_LIST_RESPONSE" ".data.list[0].id")
    if [ "$WHITELIST_ID" != "null" ] && [ "$WHITELIST_ID" != "" ]; then
        log_verbose "找到白名单ID: $WHITELIST_ID"
    else
        log_warning "未找到白名单ID，部分测试将被跳过"
    fi
fi

# 2. 获取白名单统计信息
log_info "测试获取白名单统计信息..."
api_request "GET" "$API_BASE/whitelist/stats" "" "200" "获取白名单统计信息"

# 3. 添加白名单用户
log_info "测试添加白名单用户..."
TEST_ADDRESS="test_user_$(date +%s)"
echo "TEST_ADDRESS:$TEST_ADDRESS"
api_request "POST" "$API_BASE/whitelist/" "{
    \"address\": \"$TEST_ADDRESS\",
    \"remark\": \"测试用户\",
    \"status\": 1,
    \"maxMembers\": 1000,
    \"createdBy\": \"admin_user_001\"
}" "200" "添加白名单用户"

# 4. 获取特定白名单用户信息
log_info "测试获取特定白名单用户信息..."
if [ "$WHITELIST_ID" != "null" ] && [ "$WHITELIST_ID" != "" ]; then
    api_request "GET" "$API_BASE/whitelist/info/$WHITELIST_ID" "" "200" "获取特定白名单用户信息"
else
    record_test "获取特定白名单用户信息" "SKIP" "未找到白名单ID"
fi

# 5. 更新白名单用户
log_info "测试更新白名单用户..."
if [ "$WHITELIST_ID" != "null" ] && [ "$WHITELIST_ID" != "" ]; then
    api_request "PUT" "$API_BASE/whitelist/update/$WHITELIST_ID" "{
        \"address\": \"$TEST_ADDRESS\",
        \"remark\": \"更新后的测试用户\",
        \"status\": 1,
        \"maxMembers\": 2000,
        \"createdBy\": \"admin_user_001\"
    }" "200" "更新白名单用户"
else
    record_test "更新白名单用户" "SKIP" "未找到白名单ID"
fi

# 6. 删除白名单用户
log_info "测试删除白名单用户..."
if [ "$WHITELIST_ID" != "null" ] && [ "$WHITELIST_ID" != "" ]; then
    api_request "DELETE" "$API_BASE/whitelist/delete/$WHITELIST_ID" "" "200" "删除白名单用户"
else
    record_test "删除白名单用户" "SKIP" "未找到白名单ID"
fi

# 7. 测试白名单分页查询
log_info "测试白名单分页查询..."
api_request "GET" "$API_BASE/whitelist/list?page=1&pageSize=5" "" "200" "分页查询白名单列表"




# 错误处理测试
log_info "开始错误处理测试..."

# 测试无效的群组ID
log_info "测试无效的群组ID..."
api_request "GET" "$API_BASE/groups/info/999999999" "" "500" "无效群组ID测试"

# 测试无效的请求参数
log_info "测试无效的请求参数..."
if [ "$GROUP_ID" != "null" ] && [ "$GROUP_ID" != "" ]; then
    api_request "PUT" "$API_BASE/groups/status/$GROUP_ID" '{"groupStatus": 999}' "400" "无效状态值测试"
else
    record_test "无效状态值测试" "SKIP" "未找到群组ID"
fi

# 测试缺少必需参数
log_info "测试缺少必需参数..."
if [ "$GROUP_ID" != "null" ] && [ "$GROUP_ID" != "" ]; then
    api_request "PUT" "$API_BASE/groups/name/$GROUP_ID" '{}' "400" "缺少必需参数测试"
else
    record_test "缺少必需参数测试" "SKIP" "未找到群组ID"
fi

# 性能测试
log_info "开始性能测试..."

# 测试响应时间
log_info "测试响应时间..."
start_time=$(date +%s%N)
api_request "GET" "$API_BASE/groups/list" "" "200" "响应时间测试"
end_time=$(date +%s%N)
response_time=$(( (end_time - start_time) / 1000000 ))
log_verbose "响应时间: ${response_time}ms"

# 生成测试报告
generate_report() {
    echo ""
    echo "=========================================="
    echo "测试报告"
    echo "=========================================="
    echo "总测试数: $((PASSED_TESTS + FAILED_TESTS))"
    echo "通过测试: $PASSED_TESTS"
    echo "失败测试: $FAILED_TESTS"
    echo "跳过测试: $((${#TEST_RESULTS[@]} - PASSED_TESTS - FAILED_TESTS))"
    echo ""
    
    if [ $FAILED_TESTS -eq 0 ]; then
        log_success "所有测试通过！"
        exit 0
    else
        log_error "有 $FAILED_TESTS 个测试失败"
        echo ""
        echo "失败的测试:"
        for result in "${TEST_RESULTS[@]}"; do
            IFS=':' read -r name status message <<< "$result"
            if [ "$status" = "FAIL" ]; then
                echo "  - $name: $message"
            fi
        done
        exit 1
    fi
}

# 生成测试报告
generate_report
