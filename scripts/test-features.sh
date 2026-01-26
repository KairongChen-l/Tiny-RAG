#!/bin/bash
# 功能验证测试 - 测试新添加的功能是否正常工作

set -e

echo "=== RAG系统功能验证测试 ==="
echo ""

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

PASSED=0
FAILED=0

test_pass() {
    echo -e "${GREEN}✓${NC} $1"
    ((PASSED++))
}

test_fail() {
    echo -e "${RED}✗${NC} $1"
    ((FAILED++))
}

# 1. 编译检查
echo "1. 编译检查..."
if go build -o /tmp/rag-server-test ./cmd/server 2>&1 | grep -q "error"; then
    test_fail "编译失败"
    go build -o /tmp/rag-server-test ./cmd/server 2>&1 | head -10
    exit 1
fi
test_pass "编译成功"
echo ""

# 2. 配置文件检查
echo "2. 配置文件检查..."
if [ ! -f "configs/config.yaml" ]; then
    test_fail "配置文件不存在"
    exit 1
fi

# 检查新添加的配置项
CONFIG_ITEMS=(
    "enable_query_rewrite"
    "enable_query_expand"
    "use_llm_rewriter"
    "multi_query_count"
)

for item in "${CONFIG_ITEMS[@]}"; do
    if grep -q "$item" configs/config.yaml; then
        test_pass "配置项 $item 已添加"
    else
        test_fail "配置项 $item 缺失"
    fi
done
echo ""

# 3. 代码结构检查
echo "3. 代码结构检查..."
FILES=(
    "internal/retrieval/query_rewriter.go"
    "internal/retrieval/query_rewriter_llm.go"
    "internal/metrics/prometheus.go"
    "internal/api/router.go"
)

for file in "${FILES[@]}"; do
    if [ -f "$file" ]; then
        test_pass "文件 $file 存在"
    else
        test_fail "文件 $file 不存在"
    fi
done
echo ""

# 4. 接口实现检查
echo "4. 接口实现检查..."
if grep -q "type QueryRewriter interface" internal/retrieval/query_rewriter.go; then
    test_pass "QueryRewriter接口已定义"
else
    test_fail "QueryRewriter接口未定义"
fi

if grep -q "type Metrics struct" internal/metrics/prometheus.go; then
    test_pass "Metrics结构已定义"
else
    test_fail "Metrics结构未定义"
fi
echo ""

# 5. 集成检查
echo "5. 集成检查..."
if grep -q "QueryRewriter" internal/retrieval/vector.go; then
    test_pass "查询重写器已集成到VectorRetriever"
else
    test_fail "查询重写器未集成到VectorRetriever"
fi

if grep -q "/metrics" internal/api/router.go; then
    test_pass "Metrics端点已添加到路由"
else
    test_fail "Metrics端点未添加到路由"
fi

if grep -q "metrics" internal/api/handler/handler.go; then
    test_pass "Metrics已集成到Handler"
else
    test_fail "Metrics未集成到Handler"
fi
echo ""

# 6. 启动测试（短暂）
echo "6. 服务器启动测试..."
timeout 5 /tmp/rag-server-test --config configs/config.yaml > /tmp/startup-test.log 2>&1 &
START_PID=$!
sleep 2

if ps -p $START_PID > /dev/null 2>&1; then
    # 测试健康检查
    HEALTH=$(curl -s http://localhost:8080/api/v1/health 2>/dev/null || echo "FAILED")
    if [ "$HEALTH" != "FAILED" ]; then
        test_pass "服务器启动成功，健康检查正常"
    else
        test_fail "服务器启动但健康检查失败"
    fi
    
    # 测试metrics端点
    METRICS=$(curl -s http://localhost:8080/metrics 2>/dev/null | grep -c "^rag_" || echo "0")
    if [ "$METRICS" -gt "0" ]; then
        test_pass "Metrics端点正常 (找到 $METRICS 个metrics)"
    else
        test_warn "Metrics端点可用但未找到RAG metrics"
    fi
    
    kill $START_PID 2>/dev/null || true
    wait $START_PID 2>/dev/null || true
else
    test_fail "服务器启动失败"
    cat /tmp/startup-test.log | tail -10
fi
echo ""

# 7. 单元测试检查
echo "7. 单元测试检查..."
if go test ./internal/retrieval/... -short 2>&1 | grep -q "FAIL"; then
    test_fail "检索模块测试失败"
    go test ./internal/retrieval/... -short 2>&1 | grep "FAIL" | head -5
else
    test_pass "检索模块测试通过"
fi
echo ""

# 总结
echo "=== 测试总结 ==="
echo -e "${GREEN}通过: $PASSED${NC}"
echo -e "${RED}失败: $FAILED${NC}"
echo ""

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}所有功能验证通过！${NC}"
    exit 0
else
    echo -e "${YELLOW}部分验证失败，请检查上述错误${NC}"
    exit 1
fi

