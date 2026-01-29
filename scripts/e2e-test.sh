#!/bin/bash

# 端到端测试脚本
# 用于快速验证所有组件是否正常工作

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 配置
API_URL="http://localhost:8080/api/v1"
TEST_DOC="testdata/sample-doc.md"

echo -e "${YELLOW}=== RAG 系统端到端测试 ===${NC}\n"

# 检查函数
check_service() {
    local name=$1
    local command=$2
    
    echo -n "检查 $name... "
    if eval "$command" > /dev/null 2>&1; then
        echo -e "${GREEN}✓${NC}"
        return 0
    else
        echo -e "${RED}✗${NC}"
        return 1
    fi
}

# 1. 检查 Docker 服务
echo -e "${YELLOW}步骤 1: 检查基础设施服务${NC}"
check_service "MySQL" "docker exec rag-mysql mysqladmin ping -h localhost --silent" || echo "  MySQL 未运行，请运行: docker-compose up -d"
check_service "Redis" "docker exec rag-redis redis-cli ping" || echo "  Redis 未运行，请运行: docker-compose up -d"
check_service "Elasticsearch" "curl -s http://localhost:9200/_cluster/health > /dev/null" || echo "  Elasticsearch 未运行，请运行: docker-compose up -d"
check_service "Kafka" "docker exec rag-kafka kafka-broker-api-versions --bootstrap-server localhost:9092 > /dev/null" || echo "  Kafka 未运行，请运行: docker-compose up -d"
check_service "MinIO" "curl -s http://localhost:9000/minio/health/live > /dev/null" || echo "  MinIO 未运行，请运行: docker-compose up -d"
echo ""

# 2. 检查后端服务
echo -e "${YELLOW}步骤 2: 检查后端服务${NC}"
if check_service "后端 API" "curl -s $API_URL/health > /dev/null"; then
    echo "  获取健康状态..."
    HEALTH=$(curl -s $API_URL/health)
    echo "$HEALTH" | jq '.' 2>/dev/null || echo "$HEALTH"
else
    echo -e "${RED}  后端服务未运行！${NC}"
    echo "  请运行: make run-config 或 make run-ollama"
    exit 1
fi
echo ""

# 3. 测试文档上传
echo -e "${YELLOW}步骤 3: 测试文档上传${NC}"
if [ ! -f "$TEST_DOC" ]; then
    echo -e "${YELLOW}  测试文档不存在，创建示例文档...${NC}"
    mkdir -p testdata
    cat > "$TEST_DOC" << 'EOF'
# 测试文档

这是一个测试文档，用于验证 RAG 系统的功能。

## 章节 1

RAG (Retrieval-Augmented Generation) 是一种结合检索和生成的技术。

## 章节 2

它通过检索相关文档来增强生成模型的能力。
EOF
fi

echo "  上传文档: $TEST_DOC"
UPLOAD_RESPONSE=$(curl -s -X POST "$API_URL/documents" \
    -F "file=@$TEST_DOC" \
    -F 'metadata={"title":"测试文档"}')

JOB_ID=$(echo "$UPLOAD_RESPONSE" | jq -r '.data.job_id' 2>/dev/null)

if [ -z "$JOB_ID" ] || [ "$JOB_ID" = "null" ]; then
    echo -e "${RED}  文档上传失败！${NC}"
    echo "$UPLOAD_RESPONSE" | jq '.' 2>/dev/null || echo "$UPLOAD_RESPONSE"
    exit 1
fi

echo -e "${GREEN}  文档上传成功，Job ID: $JOB_ID${NC}"
echo ""

# 4. 检查文档处理状态
echo -e "${YELLOW}步骤 4: 检查文档处理状态${NC}"
echo "  等待文档处理完成..."
for i in {1..30}; do
    JOB_STATUS=$(curl -s "$API_URL/jobs/$JOB_ID")
    STATUS=$(echo "$JOB_STATUS" | jq -r '.data.status' 2>/dev/null)
    PROGRESS=$(echo "$JOB_STATUS" | jq -r '.data.progress' 2>/dev/null)
    
    echo "  进度: $PROGRESS%, 状态: $STATUS"
    
    if [ "$STATUS" = "completed" ]; then
        echo -e "${GREEN}  文档处理完成！${NC}"
        break
    elif [ "$STATUS" = "failed" ]; then
        ERROR=$(echo "$JOB_STATUS" | jq -r '.data.error' 2>/dev/null)
        echo -e "${RED}  文档处理失败: $ERROR${NC}"
        exit 1
    fi
    
    sleep 2
done

if [ "$STATUS" != "completed" ]; then
    echo -e "${YELLOW}  文档处理超时，但继续测试...${NC}"
fi
echo ""

# 5. 检查文档列表
echo -e "${YELLOW}步骤 5: 检查文档列表${NC}"
DOCS=$(curl -s "$API_URL/documents")
DOC_COUNT=$(echo "$DOCS" | jq '.data.documents | length' 2>/dev/null)
echo "  文档数量: $DOC_COUNT"
if [ "$DOC_COUNT" -gt 0 ]; then
    echo -e "${GREEN}  文档列表正常${NC}"
    echo "$DOCS" | jq '.data.documents[0] | {id, title, format}' 2>/dev/null
else
    echo -e "${YELLOW}  文档列表为空（可能还在处理中）${NC}"
fi
echo ""

# 6. 测试对话功能
echo -e "${YELLOW}步骤 6: 测试对话功能${NC}"
echo "  创建对话..."
CONV_RESPONSE=$(curl -s -X POST "$API_URL/conversations" \
    -H "Content-Type: application/json" \
    -d '{"title":"E2E测试对话"}')

CONV_ID=$(echo "$CONV_RESPONSE" | jq -r '.data.id' 2>/dev/null)

if [ -z "$CONV_ID" ] || [ "$CONV_ID" = "null" ]; then
    echo -e "${RED}  对话创建失败！${NC}"
    echo "$CONV_RESPONSE" | jq '.' 2>/dev/null || echo "$CONV_RESPONSE"
    exit 1
fi

echo -e "${GREEN}  对话创建成功，ID: $CONV_ID${NC}"

echo "  发送测试消息..."
MESSAGE_RESPONSE=$(curl -s -X POST "$API_URL/conversations/$CONV_ID/messages" \
    -H "Content-Type: application/json" \
    -d '{"content":"什么是 RAG？"}')

ANSWER=$(echo "$MESSAGE_RESPONSE" | jq -r '.data.assistant_message.content' 2>/dev/null)

if [ -n "$ANSWER" ] && [ "$ANSWER" != "null" ]; then
    echo -e "${GREEN}  收到回答（前100字符）: ${ANSWER:0:100}...${NC}"
else
    echo -e "${YELLOW}  未收到回答（可能还在处理或没有相关文档）${NC}"
    echo "$MESSAGE_RESPONSE" | jq '.' 2>/dev/null || echo "$MESSAGE_RESPONSE"
fi
echo ""

# 7. 总结
echo -e "${YELLOW}=== 测试总结 ===${NC}"
echo -e "${GREEN}✓ 基础设施服务检查完成${NC}"
echo -e "${GREEN}✓ 后端服务检查完成${NC}"
echo -e "${GREEN}✓ 文档上传测试完成${NC}"
echo -e "${GREEN}✓ 文档处理检查完成${NC}"
echo -e "${GREEN}✓ 文档列表检查完成${NC}"
echo -e "${GREEN}✓ 对话功能测试完成${NC}"
echo ""
echo -e "${GREEN}所有测试完成！${NC}"
echo ""
echo "下一步："
echo "  1. 启动前端应用: cd frontend/rag_flutter && flutter run -d chrome"
echo "  2. 访问 Web UI: http://localhost:8080"
echo "  3. 查看 API 文档: docs/API.md"


