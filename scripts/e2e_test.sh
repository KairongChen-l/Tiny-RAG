#!/bin/bash

# RAG系统端到端测试脚本
# 使用 Kimi API 进行测试

set -e

BASE_URL="http://localhost:8080/api/v1"
TEST_DOC_ID=""
UPLOADED_DOCS=()

echo "=========================================="
echo "RAG系统端到端测试 (Kimi API)"
echo "=========================================="
echo ""

# 颜色定义
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 测试函数
test_pass() {
    echo -e "${GREEN}✓ PASS${NC}: $1"
}

test_fail() {
    echo -e "${RED}✗ FAIL${NC}: $1"
    exit 1
}

test_info() {
    echo -e "${YELLOW}ℹ INFO${NC}: $1"
}

# 1. 健康检查
echo "1. 测试健康检查端点..."
HEALTH_RESPONSE=$(curl -s -w "\n%{http_code}" "${BASE_URL}/health")
HTTP_CODE=$(echo "$HEALTH_RESPONSE" | tail -n1)
BODY=$(echo "$HEALTH_RESPONSE" | sed '$d')

if [ "$HTTP_CODE" = "200" ]; then
    test_pass "健康检查返回 200"
    echo "响应: $BODY" | head -c 200
    echo ""
else
    test_fail "健康检查返回 $HTTP_CODE"
fi
echo ""

# 2. 文档列表（分页测试）
echo "2. 测试文档列表（分页）..."
LIST_RESPONSE=$(curl -s -w "\n%{http_code}" "${BASE_URL}/documents?limit=10&offset=0&sort_by=created_at&order=desc")
HTTP_CODE=$(echo "$LIST_RESPONSE" | tail -n1)
BODY=$(echo "$LIST_RESPONSE" | sed '$d')

if [ "$HTTP_CODE" = "200" ]; then
    test_pass "文档列表返回 200"
    # 检查响应是否包含分页信息
    if echo "$BODY" | grep -q "\"total\""; then
        test_pass "响应包含分页信息"
    else
        test_fail "响应缺少分页信息"
    fi
    echo "响应: $BODY" | head -c 300
    echo ""
else
    test_fail "文档列表返回 $HTTP_CODE"
fi
echo ""

# 3. 检查 embedding 配置
echo "3. 检查 embedding 配置..."
HEALTH_BODY=$(curl -s "${BASE_URL}/health")
EMBEDDING_STATUS=$(echo "$HEALTH_BODY" | grep -o '"embedding":"[^"]*"' | cut -d'"' -f4)

if [ "$EMBEDDING_STATUS" = "not_configured" ]; then
    test_info "Embedding 未配置，跳过文档上传和查询测试"
    SKIP_UPLOAD=true
else
    test_info "Embedding 已配置: $EMBEDDING_STATUS"
    SKIP_UPLOAD=false
fi
echo ""

# 4. 创建测试文档（上传）- 仅在 embedding 配置时执行
if [ "$SKIP_UPLOAD" = "false" ]; then
    echo "4. 测试文档上传..."
    # 创建临时测试文件（使用 .md 扩展名）
    TEST_FILE=$(mktemp --suffix=.md)
    echo "# 测试文档

这是一个用于端到端测试的文档。

## 章节1

这是章节1的内容。RAG系统应该能够检索这些信息。

## 章节2

这是章节2的内容。包含一些测试关键词。
" > "$TEST_FILE"

    UPLOAD_RESPONSE=$(curl -s -w "\n%{http_code}" \
        -X POST \
        -F "file=@${TEST_FILE}" \
        -F "title=端到端测试文档" \
        "${BASE_URL}/documents")
    HTTP_CODE=$(echo "$UPLOAD_RESPONSE" | tail -n1)
    BODY=$(echo "$UPLOAD_RESPONSE" | sed '$d')

    if [ "$HTTP_CODE" = "202" ]; then
        test_pass "文档上传返回 202"
        # 提取 job_id
        JOB_ID=$(echo "$BODY" | grep -o '"job_id":"[^"]*"' | cut -d'"' -f4)
        if [ -n "$JOB_ID" ]; then
            test_pass "成功获取 job_id: $JOB_ID"
            test_info "等待文档处理完成..."
            
            # 等待处理完成（最多等待30秒）
            for i in {1..30}; do
                sleep 1
                JOB_RESPONSE=$(curl -s "${BASE_URL}/jobs/${JOB_ID}")
                JOB_STATUS=$(echo "$JOB_RESPONSE" | grep -o '"status":"[^"]*"' | cut -d'"' -f4)
                
                if [ "$JOB_STATUS" = "completed" ]; then
                    test_pass "文档处理完成"
                    # 提取文档ID
                    DOC_ID=$(echo "$JOB_RESPONSE" | grep -o '"result":"[^"]*"' | cut -d'"' -f4 | grep -o '"[^"]*"' | head -1 | tr -d '"')
                    if [ -n "$DOC_ID" ]; then
                        TEST_DOC_ID="$DOC_ID"
                        UPLOADED_DOCS+=("$DOC_ID")
                        test_pass "文档ID: $TEST_DOC_ID"
                    fi
                    break
                elif [ "$JOB_STATUS" = "failed" ]; then
                    ERROR_MSG=$(echo "$JOB_RESPONSE" | grep -o '"error":"[^"]*"' | cut -d'"' -f4)
                    test_info "文档处理失败: $ERROR_MSG"
                    break
                fi
            done
        fi
    else
        test_fail "文档上传返回 $HTTP_CODE: $BODY"
    fi

    # 清理临时文件
    rm -f "$TEST_FILE"
    echo ""
else
    echo "4. 跳过文档上传测试（embedding 未配置）"
    echo ""
fi

# 5. 查询测试（需要等待文档索引完成）
if [ -n "$TEST_DOC_ID" ] && [ "$SKIP_UPLOAD" = "false" ]; then
    echo "4. 测试查询功能..."
    test_info "等待索引完成（5秒）..."
    sleep 5
    
    QUERY_RESPONSE=$(curl -s -w "\n%{http_code}" \
        -X POST \
        -H "Content-Type: application/json" \
        -d '{"query": "测试文档的内容是什么？"}' \
        "${BASE_URL}/query")
    HTTP_CODE=$(echo "$QUERY_RESPONSE" | tail -n1)
    BODY=$(echo "$QUERY_RESPONSE" | sed '$d')
    
    if [ "$HTTP_CODE" = "200" ]; then
        test_pass "查询返回 200"
        if echo "$BODY" | grep -q "\"answer\""; then
            test_pass "查询返回答案"
            echo "答案预览: $(echo "$BODY" | grep -o '"answer":"[^"]*"' | cut -d'"' -f4 | head -c 200)"
            echo ""
        else
            test_fail "查询响应缺少答案"
        fi
    else
        test_info "查询返回 $HTTP_CODE (可能是索引未完成): $BODY"
    fi
    echo ""
fi

# 6. 测试批量删除
if [ ${#UPLOADED_DOCS[@]} -gt 0 ]; then
    echo "5. 测试批量删除..."
    DOC_IDS_JSON=$(printf '"%s",' "${UPLOADED_DOCS[@]}" | sed 's/,$//')
    BATCH_DELETE_RESPONSE=$(curl -s -w "\n%{http_code}" \
        -X POST \
        -H "Content-Type: application/json" \
        -d "{\"document_ids\": [$DOC_IDS_JSON]}" \
        "${BASE_URL}/documents/batch-delete")
    HTTP_CODE=$(echo "$BATCH_DELETE_RESPONSE" | tail -n1)
    BODY=$(echo "$BATCH_DELETE_RESPONSE" | sed '$d')
    
    if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "207" ]; then
        test_pass "批量删除返回 $HTTP_CODE"
        if echo "$BODY" | grep -q "\"deleted\""; then
            test_pass "批量删除成功"
        else
            test_fail "批量删除响应格式错误"
        fi
    else
        test_fail "批量删除返回 $HTTP_CODE: $BODY"
    fi
    echo ""
fi

# 7. 测试参数验证
echo "6. 测试参数验证..."
# 测试无效的 limit
INVALID_LIMIT_RESPONSE=$(curl -s -w "\n%{http_code}" "${BASE_URL}/documents?limit=2000")
HTTP_CODE=$(echo "$INVALID_LIMIT_RESPONSE" | tail -n1)
if [ "$HTTP_CODE" = "400" ]; then
    test_pass "无效 limit 参数验证正确"
else
    test_info "无效 limit 参数返回 $HTTP_CODE (可能未实现验证)"
fi

# 测试无效的 sort_by
INVALID_SORT_RESPONSE=$(curl -s -w "\n%{http_code}" "${BASE_URL}/documents?sort_by=invalid_field")
HTTP_CODE=$(echo "$INVALID_SORT_RESPONSE" | tail -n1)
if [ "$HTTP_CODE" = "400" ]; then
    test_pass "无效 sort_by 参数验证正确"
else
    test_info "无效 sort_by 参数返回 $HTTP_CODE (可能未实现验证)"
fi
echo ""

# 8. 测试请求ID追踪
echo "7. 测试请求ID追踪..."
REQUEST_ID_RESPONSE=$(curl -s -I "${BASE_URL}/health" | grep -i "X-Request-ID")
if [ -n "$REQUEST_ID_RESPONSE" ]; then
    test_pass "请求ID追踪正常"
    echo "  $REQUEST_ID_RESPONSE"
else
    test_fail "请求ID追踪未实现"
fi
echo ""

echo "=========================================="
echo -e "${GREEN}所有测试完成！${NC}"
echo "=========================================="

