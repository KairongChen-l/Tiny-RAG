# RAG系统 API 文档

## 概述

RAG系统提供RESTful API用于文档管理、查询和对话功能。

**Base URL**: `http://localhost:8080/api/v1`

## 认证

当前版本不需要认证，生产环境建议添加API密钥认证。

## 通用响应格式

### 成功响应

```json
{
  "data": { ... }
}
```

### 错误响应

```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "Error message",
    "details": { ... },
    "retryable": false
  }
}
```

## 端点列表

### 健康检查

#### GET /health

检查服务健康状态。

**响应**:
```json
{
  "status": "ok",
  "version": "1.0.0",
  "database": "connected",
  "embedding": "configured",
  "llm": "configured"
}
```

### 文档管理

#### POST /documents

上传单个文档。

**请求**:
- Content-Type: `multipart/form-data`
- 字段:
  - `file`: 文件（必需）
  - `title`: 文档标题（可选）
  - `metadata`: JSON格式的元数据（可选）

**响应**:
```json
{
  "job_id": "uuid",
  "status": "pending",
  "message": "Document processing started"
}
```

#### POST /documents/batch

批量上传文档。

**请求**:
- Content-Type: `multipart/form-data`
- 字段:
  - `files`: 多个文件（用于多文件上传）
  - `zip`: ZIP压缩包（用于ZIP文件上传）
  - `metadata`: 全局元数据（可选）

**响应**:
```json
{
  "jobs": [
    {
      "job_id": "uuid",
      "filename": "doc1.txt",
      "status": "pending"
    }
  ],
  "total": 2,
  "success": 2,
  "failed": 0
}
```

#### GET /documents

列出文档。

**查询参数**:
- `limit`: 每页数量（默认50，最大1000）
- `offset`: 偏移量（默认0）
- `sort_by`: 排序字段（`created_at`, `updated_at`, `title`）
- `order`: 排序顺序（`asc`, `desc`）
- `include_deleted`: 包含已删除文档（`true`/`false`）
- `search`: 全文搜索查询
- `filter_<key>`: 元数据过滤（如`filter_author=John`）

**响应**:
```json
{
  "documents": [
    {
      "id": "doc-id",
      "source": "file://path",
      "title": "Document Title",
      "format": "markdown",
      "created_at": "2026-01-27 10:00:00",
      "updated_at": "2026-01-27 10:00:00",
      "metadata": {}
    }
  ],
  "total": 100,
  "limit": 50,
  "offset": 0
}
```

#### DELETE /documents/{id}

删除文档（软删除）。

**查询参数**:
- `hard`: 是否硬删除（`true`/`false`，默认`false`）

**响应**:
```json
{
  "message": "document deleted"
}
```

#### POST /documents/{id}/restore

恢复软删除的文档。

**响应**:
```json
{
  "message": "document restored"
}
```

#### GET /documents/{id}/versions

列出文档版本。

**响应**:
```json
{
  "document_id": "doc-id",
  "versions": [
    {
      "version": 1,
      "hash": "abc123",
      "chunk_count": 10,
      "created_at": "2026-01-27 10:00:00",
      "change_note": "Initial version"
    }
  ]
}
```

#### POST /documents/{id}/restore-version

恢复文档到指定版本。

**请求体**:
```json
{
  "version": 1
}
```

**响应**:
```json
{
  "message": "document version restored",
  "document_id": "doc-id",
  "version": 1
}
```

#### POST /documents/batch/delete

批量删除文档。

**请求体**:
```json
{
  "document_ids": ["doc1", "doc2"],
  "hard": false
}
```

**响应**:
```json
{
  "deleted": ["doc1", "doc2"],
  "failed": [],
  "total": 2,
  "success": 2,
  "failed_count": 0
}
```

#### GET /documents/stats

获取文档统计信息。

**响应**:
```json
{
  "total_documents": 100,
  "total_chunks": 500,
  "total_size": 1024000
}
```

### 查询

#### POST /query

执行RAG查询。

**请求体**:
```json
{
  "query": "What is Go?",
  "top_k": 5,
  "include_citations": true
}
```

**响应**:
```json
{
  "answer": "Go is a programming language...",
  "citations": [
    {
      "index": 1,
      "chunk_id": "chunk-id",
      "content": "Go (Golang) is...",
      "document_id": "doc-id",
      "score": 0.95
    }
  ],
  "query_used": "What is Go programming language?"
}
```

### 对话

#### POST /conversations

创建新对话。

**请求体**:
```json
{
  "title": "Conversation Title"
}
```

**响应**:
```json
{
  "id": "conv-id",
  "title": "Conversation Title",
  "created_at": "2026-01-27 10:00:00"
}
```

#### POST /conversations/{id}/messages

发送消息。

**请求体**:
```json
{
  "message": "What is RAG?",
  "stream": false
}
```

**响应**:
```json
{
  "message_id": "msg-id",
  "response": "RAG stands for...",
  "citations": []
}
```

### 任务管理

#### GET /jobs/{id}

获取任务状态。

**响应**:
```json
{
  "id": "job-id",
  "type": "document_ingest",
  "status": "completed",
  "progress": 100,
  "current_stage": "completed",
  "stage_message": "Document processed successfully",
  "progress_history": [
    {
      "progress": 50,
      "stage": "chunking",
      "stage_message": "Processing chunks",
      "timestamp": "2026-01-27T10:00:00Z"
    }
  ]
}
```

## 错误码

- `BAD_REQUEST`: 请求参数错误
- `NOT_FOUND`: 资源不存在
- `INTERNAL_ERROR`: 服务器内部错误
- `RATE_LIMIT_EXCEEDED`: 请求频率超限
- `UNSUPPORTED_FORMAT`: 不支持的文件格式
- `VALIDATION`: 数据验证失败

## 速率限制

如果启用了速率限制，超过限制的请求将返回：
- 状态码: `429 Too Many Requests`
- Header: `Retry-After: <seconds>`

## 示例

### 上传文档

```bash
curl -X POST http://localhost:8080/api/v1/documents \
  -F "file=@document.md" \
  -F "title=My Document"
```

### 查询

```bash
curl -X POST http://localhost:8080/api/v1/query \
  -H "Content-Type: application/json" \
  -d '{"query": "What is Go?"}'
```

### 批量上传

```bash
curl -X POST http://localhost:8080/api/v1/documents/batch \
  -F "files=@doc1.txt" \
  -F "files=@doc2.md"
```

