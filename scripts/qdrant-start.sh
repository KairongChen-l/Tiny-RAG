#!/bin/bash
# 启动Qdrant容器

cd "$(dirname "$0")/.."

if docker ps --filter "name=qdrant" --format "{{.Names}}" 2>/dev/null | grep -q qdrant; then
    echo "Qdrant容器已在运行"
    docker ps --filter "name=qdrant" --format "  容器: {{.Names}}\n  状态: {{.Status}}\n  端口: {{.Ports}}"
    exit 0
fi

if docker ps -a --filter "name=qdrant" --format "{{.Names}}" 2>/dev/null | grep -q qdrant; then
    echo "启动已存在的Qdrant容器..."
    docker start qdrant
else
    echo "创建新的Qdrant容器..."
    mkdir -p qdrant_storage
    docker run -d --name qdrant -p 6333:6333 -p 6334:6334 \
        -v "$(pwd)/qdrant_storage:/qdrant/storage" \
        qdrant/qdrant:latest
fi

echo "等待Qdrant启动..."
sleep 3

for i in {1..10}; do
    if curl -s http://localhost:6333/health >/dev/null 2>&1; then
        echo "✅ Qdrant服务已启动"
        break
    fi
    echo "等待中 ($i/10)..."
    sleep 2
done

curl -s http://localhost:6333/health | python3 -c "import sys, json; d=json.load(sys.stdin); print(f'健康状态: {d.get(\"status\", \"unknown\")}')" 2>/dev/null || echo "检查服务状态..."
