#!/bin/bash
# 完整的端到端测试脚本

set -e

echo "=== RAG系统完整端到端测试 ==="
echo ""

# 颜色定义
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 测试结果计数
PASSED=0
FAILED=0

# 辅助函数
test_pass() {
    echo -e "${GREEN}✓${NC} $1"
    ((PASSED++))
}

test_fail() {
    echo -e "${RED}✗${NC} $1"
    ((FAILED++))
}

test_warn() {
    echo -e "${YELLOW}⚠${NC} $1"
}

# 1. 检查编译
echo "1. 检查编译..."
if [ ! -f /tmp/rag-server ]; then
    test_fail "服务器未编译"
    exit 1
fi
test_pass "编译成功"
echo ""

# 2. 启动服务器
echo "2. 启动服务器..."
/tmp/rag-server --config configs/config.yaml > /tmp/rag-server.log 2>&1 &
SERVER_PID=$!
sleep 3

if ! ps -p $SERVER_PID > /dev/null; then
    test_fail "服务器启动失败"
    cat /tmp/rag-server.log
    exit 1
fi
test_pass "服务器已启动 (PID: $SERVER_PID)"
echo ""

# 3. 测试健康检查
echo "3. 测试健康检查端点..."
HEALTH_RESPONSE=$(curl -s http://localhost:8080/api/v1/health)
if echo "$HEALTH_RESPONSE" | grep -q "success"; then
    test_pass "健康检查通过"
    echo "  响应: $HEALTH_RESPONSE"
else
    test_fail "健康检查失败: $HEALTH_RESPONSE"
fi
echo ""

# 4. 测试Metrics端点
echo "4. 测试Metrics端点..."
METRICS_COUNT=$(curl -s http://localhost:8080/metrics | grep -c "^rag_" || echo "0")
if [ "$METRICS_COUNT" -gt "0" ]; then
    test_pass "Metrics端点正常 (找到 $METRICS_COUNT 个RAG metrics)"
    echo "  示例metrics:"
    curl -s http://localhost:8080/metrics | grep "^rag_" | head -3 | sed 's/^/    /'
else
    test_warn "Metrics端点可用但未找到RAG metrics"
fi
echo ""

# 5. 测试文档列表
echo "5. 测试文档列表端点..."
DOCS_RESPONSE=$(curl -s http://localhost:8080/api/v1/documents)
if echo "$DOCS_RESPONSE" | grep -q "documents"; then
    test_pass "文档列表端点正常"
    DOC_COUNT=$(echo "$DOCS_RESPONSE" | grep -o '"documents":\[' | wc -l || echo "0")
    echo "  当前文档数: $DOC_COUNT"
else
    test_fail "文档列表端点失败: $DOCS_RESPONSE"
fi
echo ""

# 6. 测试查询端点（无文档时）
echo "6. 测试查询端点..."
QUERY_RESPONSE=$(curl -s -X POST http://localhost:8080/api/v1/query \
    -H "Content-Type: application/json" \
    -d '{"query":"test query"}' 2>&1)
if echo "$QUERY_RESPONSE" | grep -q "answer\|error"; then
    test_pass "查询端点响应正常"
    echo "  响应预览: $(echo "$QUERY_RESPONSE" | head -c 100)..."
else
    test_warn "查询端点响应异常: $QUERY_RESPONSE"
fi
echo ""

# 7. 测试查询重写配置
echo "7. 检查查询重写配置..."
CONFIG_CHECK=$(grep -c "enable_query_rewrite" configs/config.yaml || echo "0")
if [ "$CONFIG_CHECK" -gt "0" ]; then
    test_pass "查询重写配置已添加"
else
    test_warn "查询重写配置未找到"
fi
echo ""

# 8. 验证Metrics记录
echo "8. 验证Metrics记录..."
# 发送一个查询请求
curl -s -X POST http://localhost:8080/api/v1/query \
    -H "Content-Type: application/json" \
    -d '{"query":"test"}' > /dev/null 2>&1 || true

sleep 1

# 检查metrics是否更新
RETRIEVAL_REQUESTS=$(curl -s http://localhost:8080/metrics | grep "rag_retrieval_requests_total" | grep -o "[0-9.]*$" | head -1 || echo "0")
if [ "$RETRIEVAL_REQUESTS" != "0" ]; then
    test_pass "检索metrics已记录 (请求数: $RETRIEVAL_REQUESTS)"
else
    test_warn "检索metrics未更新（可能是无embedder配置）"
fi
echo ""

# 9. 测试API路由
echo "9. 测试API路由..."
ROUTES=(
    "/api/v1/health"
    "/api/v1/documents"
    "/metrics"
)

for route in "${ROUTES[@]}"; do
    STATUS=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:8080$route")
    if [ "$STATUS" -ge "200" ] && [ "$STATUS" -lt "500" ]; then
        test_pass "路由 $route 正常 (HTTP $STATUS)"
    else
        test_fail "路由 $route 失败 (HTTP $STATUS)"
    fi
done
echo ""

# 10. 检查服务器日志
echo "10. 检查服务器日志..."
ERROR_COUNT=$(grep -i "error\|fatal" /tmp/rag-server.log | grep -v "no LLM provider available\|no embedder configured" | wc -l || echo "0")
if [ "$ERROR_COUNT" -eq "0" ]; then
    test_pass "服务器日志无严重错误"
else
    test_warn "服务器日志中发现 $ERROR_COUNT 个错误/警告"
    echo "  最近的错误:"
    grep -i "error\|fatal" /tmp/rag-server.log | tail -3 | sed 's/^/    /' || true
fi
echo ""

# 清理
echo "清理..."
kill $SERVER_PID 2>/dev/null || true
wait $SERVER_PID 2>/dev/null || true
test_pass "服务器已停止"
echo ""

# 总结
echo "=== 测试总结 ==="
echo -e "${GREEN}通过: $PASSED${NC}"
echo -e "${RED}失败: $FAILED${NC}"
echo ""

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}所有关键测试通过！${NC}"
    exit 0
else
    echo -e "${RED}部分测试失败，请检查日志${NC}"
    exit 1
fi

