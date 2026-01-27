#!/bin/bash

echo "=== 启动RAG系统 ==="
echo ""

# 1. 检查Ollama
echo "1. 检查Ollama..."
if ! curl -s http://localhost:11434/api/tags > /dev/null 2>&1; then
    echo "⚠️  Ollama未运行，请先启动: ollama serve"
    echo "   或者按Enter继续（如果Ollama在其他位置运行）"
    read
else
    echo "✓ Ollama运行中"
fi

# 2. 检查模型
echo ""
echo "2. 检查必要的模型..."
MODELS=$(ollama list 2>/dev/null | grep -E "nomic-embed-text|llama2" | wc -l)
if [ "$MODELS" -lt "2" ]; then
    echo "⚠️  缺少必要的模型，正在下载..."
    ollama pull nomic-embed-text
    ollama pull llama2
else
    echo "✓ 模型已就绪"
fi

# 3. 启动后端
echo ""
echo "3. 启动后端服务器..."
cd /home/krc/RAG
pkill -f "rag-server|go run.*cmd/server" 2>/dev/null
sleep 1

go run ./cmd/server --config configs/config-ollama.yaml > /tmp/rag-server.log 2>&1 &
BACKEND_PID=$!
sleep 4

if curl -s http://localhost:8080/api/v1/health > /dev/null 2>&1; then
    echo "✓ 后端运行中 (PID: $BACKEND_PID)"
    echo "  日志: /tmp/rag-server.log"
else
    echo "✗ 后端启动失败，查看日志: /tmp/rag-server.log"
    tail -20 /tmp/rag-server.log
    exit 1
fi

# 4. 启动Flutter前端
echo ""
echo "4. 启动Flutter前端..."
cd frontend/rag_flutter

if ! command -v flutter &> /dev/null; then
    echo "⚠️  Flutter未安装，跳过前端启动"
    echo "   请手动运行: cd frontend/rag_flutter && flutter run -d chrome"
else
    flutter run -d chrome > /tmp/flutter.log 2>&1 &
    FLUTTER_PID=$!
    echo "✓ Flutter启动中 (PID: $FLUTTER_PID)"
    echo "  日志: /tmp/flutter.log"
fi

echo ""
echo "=== 启动完成 ==="
echo "后端API: http://localhost:8080"
echo "健康检查: http://localhost:8080/api/v1/health"
echo ""
echo "停止服务:"
echo "  pkill -f 'rag-server|go run.*cmd/server'"
echo "  pkill -f flutter"
