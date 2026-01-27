#!/bin/bash
# 停止Qdrant容器

if docker ps --filter "name=qdrant" --format "{{.Names}}" 2>/dev/null | grep -q qdrant; then
    docker stop qdrant
    echo "✅ Qdrant容器已停止"
else
    echo "Qdrant容器未运行"
fi
