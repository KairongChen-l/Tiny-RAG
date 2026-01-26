#!/bin/bash
# 端到端测试脚本

set -e

echo "=== RAG系统端到端测试 ==="
echo ""

# 1. 检查服务器是否编译成功
echo "1. 检查编译..."
if [ ! -f /tmp/rag-server ]; then
    echo "错误: 服务器未编译"
    exit 1
fi
echo "✓ 编译成功"
echo ""

# 2. 启动服务器（后台）
echo "2. 启动服务器..."
/tmp/rag-server --config configs/config.yaml > /tmp/rag-server.log 2>&1 &
SERVER_PID=$!
sleep 3

# 检查服务器是否启动
if ! ps -p $SERVER_PID > /dev/null; then
    echo "错误: 服务器启动失败"
    cat /tmp/rag-server.log
    exit 1
fi
echo "✓ 服务器已启动 (PID: $SERVER_PID)"
echo ""

# 3. 测试健康检查端点
echo "3. 测试健康检查端点..."
HEALTH_RESPONSE=$(curl -s http://localhost:8080/api/v1/health || echo "FAILED")
if [ "$HEALTH_RESPONSE" = "FAILED" ]; then
    echo "错误: 健康检查失败"
    cat /tmp/rag-server.log
    kill $SERVER_PID 2>/dev/null || true
    exit 1
fi
echo "✓ 健康检查通过: $HEALTH_RESPONSE"
echo ""

# 4. 测试Metrics端点
echo "4. 测试Metrics端点..."
METRICS_RESPONSE=$(curl -s http://localhost:8080/metrics | head -5 || echo "FAILED")
if [ "$METRICS_RESPONSE" = "FAILED" ]; then
    echo "警告: Metrics端点不可用"
else
    echo "✓ Metrics端点正常"
    echo "  示例输出:"
    echo "$METRICS_RESPONSE" | head -3
fi
echo ""

# 5. 测试文档列表端点
echo "5. 测试文档列表端点..."
DOCS_RESPONSE=$(curl -s http://localhost:8080/api/v1/documents || echo "FAILED")
if [ "$DOCS_RESPONSE" = "FAILED" ]; then
    echo "警告: 文档列表端点失败"
else
    echo "✓ 文档列表端点正常"
fi
echo ""

# 6. 清理
echo "6. 清理..."
kill $SERVER_PID 2>/dev/null || true
wait $SERVER_PID 2>/dev/null || true
echo "✓ 服务器已停止"
echo ""

echo "=== 测试完成 ==="

