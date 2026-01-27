#!/bin/bash

echo "=== 停止RAG系统 ==="

# 停止后端
echo "停止后端服务器..."
pkill -f "rag-server|go run.*cmd/server" 2>/dev/null
if [ $? -eq 0 ]; then
    echo "✓ 后端已停止"
else
    echo "  后端未运行"
fi

# 停止Flutter
echo "停止Flutter前端..."
pkill -f flutter 2>/dev/null
if [ $? -eq 0 ]; then
    echo "✓ Flutter已停止"
else
    echo "  Flutter未运行"
fi

echo ""
echo "✓ 所有服务已停止"
