#!/bin/bash

# RAG 系统完整测试脚本
# 使用方法: ./test.sh

set -e

API_BASE="http://localhost:8080/api/v1"
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}=== RAG 系统完整测试 ===${NC}\n"

# 检查服务器是否运行
echo "1. 检查服务器状态..."
if ! curl -s "${API_BASE}/health" > /dev/null; then
    echo -e "${RED}错误: 服务器未运行，请先启动服务器:${NC}"
    echo "   make run-ollama"
    exit 1
fi
echo -e "${GREEN}✓ 服务器运行正常${NC}\n"

# 健康检查
echo "2. 健康检查..."
HEALTH=$(curl -s "${API_BASE}/health")
echo "$HEALTH" | jq '.'
if echo "$HEALTH" | jq -e '.success == true' > /dev/null; then
    echo -e "${GREEN}✓ 健康检查通过${NC}\n"
else
    echo -e "${RED}✗ 健康检查失败${NC}\n"
    exit 1
fi

# 检查初始文档列表
echo "3. 检查初始文档列表..."
DOCS=$(curl -s "${API_BASE}/documents")
DOC_COUNT=$(echo "$DOCS" | jq '.data.documents | length')
echo "当前文档数量: $DOC_COUNT"
echo "$DOCS" | jq '.data.documents[] | {id, source, title, format}' 2>/dev/null || echo "无文档"
echo ""

# 上传文档
echo "4. 上传测试文档..."
if [ ! -f "testdata/sample-doc.md" ]; then
    echo -e "${RED}错误: 测试文件不存在: testdata/sample-doc.md${NC}"
    exit 1
fi

UPLOAD_RESPONSE=$(curl -s -X POST "${API_BASE}/documents" \
    -F "file=@testdata/sample-doc.md")

echo "$UPLOAD_RESPONSE" | jq '.'

if echo "$UPLOAD_RESPONSE" | jq -e '.success == true' > /dev/null; then
    JOB_ID=$(echo "$UPLOAD_RESPONSE" | jq -r '.data.job_id')
    echo -e "${GREEN}✓ 文档上传成功，Job ID: $JOB_ID${NC}\n"
else
    echo -e "${RED}✗ 文档上传失败${NC}\n"
    exit 1
fi

# 等待文档处理（最多等待60秒）
echo "5. 等待文档处理..."
FAILED=false
FINAL_STATUS=""
for i in {1..60}; do
    sleep 1
    JOB_STATUS=$(curl -s "${API_BASE}/jobs/${JOB_ID}")
    STATUS=$(echo "$JOB_STATUS" | jq -r '.data.status // "unknown"')
    PROGRESS=$(echo "$JOB_STATUS" | jq -r '.data.progress // 0')
    FINAL_STATUS="$STATUS"
    
    echo -ne "\r   进度: ${PROGRESS}% (状态: ${STATUS})"
    
    if [ "$STATUS" = "completed" ]; then
        echo -e "\n${GREEN}✓ 文档处理完成${NC}\n"
        break
    elif [ "$STATUS" = "failed" ]; then
        ERROR=$(echo "$JOB_STATUS" | jq -r '.data.error // "Unknown error"')
        echo -e "\n${RED}✗ 文档处理失败${NC}"
        echo -e "${RED}错误详情: $ERROR${NC}"
        echo -e "\n完整 Job 状态:"
        echo "$JOB_STATUS" | jq '.'
        echo ""
        
        # 提供诊断建议
        if echo "$ERROR" | grep -qi "embedder\|embed"; then
            echo -e "${YELLOW}诊断建议:${NC}"
            echo "  - 检查 embedder 是否已正确配置"
            echo "  - 如果使用 Ollama，确保服务正在运行: ollama serve"
            echo "  - 检查模型是否已下载: ollama list"
            echo "  - 查看服务器日志获取更多信息"
        fi
        FAILED=true
        break
    fi
    
    if [ $i -eq 60 ]; then
        echo -e "\n${YELLOW}⚠ 文档处理超时（60秒）${NC}"
        echo "当前状态:"
        echo "$JOB_STATUS" | jq '.'
        echo ""
    fi
done

if [ "$FAILED" = true ]; then
    exit 1
fi

# 检查文档列表（应该能看到新上传的文档）
if [ "$FAILED" != true ]; then
    echo "6. 检查文档列表..."
    sleep 2  # 等待数据库更新
    DOCS=$(curl -s "${API_BASE}/documents")
    NEW_DOC_COUNT=$(echo "$DOCS" | jq '.data.documents | length')
    echo "文档数量: $NEW_DOC_COUNT"

    if [ "$NEW_DOC_COUNT" -gt "$DOC_COUNT" ]; then
        echo -e "${GREEN}✓ 文档已添加到列表${NC}"
        echo "$DOCS" | jq '.data.documents[] | {id, source, title, format, created_at}'
    else
        echo -e "${YELLOW}⚠ 文档列表未更新（可能仍在处理中）${NC}"
    fi
    echo ""
else
    echo -e "${YELLOW}6. 跳过文档列表检查（文档处理失败）${NC}\n"
fi

# 创建对话
echo "7. 创建对话..."
CONV_RESPONSE=$(curl -s -X POST "${API_BASE}/conversations" \
    -H "Content-Type: application/json" \
    -d '{"title":"测试对话"}')

CONV_ID=$(echo "$CONV_RESPONSE" | jq -r '.data.id')
echo "对话 ID: $CONV_ID"

if [ "$CONV_ID" != "null" ] && [ -n "$CONV_ID" ]; then
    echo -e "${GREEN}✓ 对话创建成功${NC}\n"
else
    echo -e "${RED}✗ 对话创建失败${NC}\n"
    exit 1
fi

# 发送消息（如果文档已处理完成）
if [ "$FAILED" != true ] && [ "$FINAL_STATUS" = "completed" ]; then
    echo "8. 发送测试消息..."
    MESSAGE_RESPONSE=$(curl -s -X POST "${API_BASE}/conversations/${CONV_ID}/messages" \
        -H "Content-Type: application/json" \
        -d '{"content":"什么是 RAG 系统？"}')
    
    echo "$MESSAGE_RESPONSE" | jq '.data.assistant_message.content' 2>/dev/null || echo "$MESSAGE_RESPONSE" | jq '.'
    
    if echo "$MESSAGE_RESPONSE" | jq -e '.success == true' > /dev/null; then
        echo -e "${GREEN}✓ 消息发送成功${NC}\n"
    else
        ERROR=$(echo "$MESSAGE_RESPONSE" | jq -r '.error.message // "Unknown error"')
        echo -e "${YELLOW}⚠ 消息发送可能失败: $ERROR${NC}"
        echo "  （这可能是正常的，如果 embedder/LLM 未配置）\n"
    fi
else
    echo -e "${YELLOW}8. 跳过消息测试（文档未处理完成）${NC}\n"
fi

# 列出所有对话
echo "9. 列出所有对话..."
CONVS=$(curl -s "${API_BASE}/conversations")
CONV_COUNT=$(echo "$CONVS" | jq '.data.conversations | length')
echo "对话数量: $CONV_COUNT"
echo "$CONVS" | jq '.data.conversations[] | {id, title, created_at}' 2>/dev/null || echo "无对话"
echo ""

echo -e "${GREEN}=== 测试完成 ===${NC}"
echo ""
echo "前端界面: http://localhost:8080"
echo "API 文档: 查看 README.md"

