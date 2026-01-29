# HTTP API 模块 (API)

## 模块职责

HTTP API 模块提供 RESTful API 接口，是系统对外的统一入口。负责请求路由、参数验证、错误处理和响应格式化。

## 核心功能

- **RESTful API**：标准的 REST 接口设计
- **请求验证**：参数校验和内容类型检查
- **错误处理**：统一的错误响应格式
- **中间件**：日志、限流、请求ID等
- **健康检查**：系统健康状态监控

## API 端点

### 健康检查

```
GET /api/v1/health
```

**响应**：
```json
{
  "success": true,
  "data": {
    "status": "healthy",
    "components": {
      "database": "ok",
      "embedding": "ok",
      "llm": "ok"
    }
  }
}
```

### 文档管理

#### 上传文档

```
POST /api/v1/documents
Content-Type: multipart/form-data
```

**请求**：
- `file`: 文档文件
- `title` (可选): 文档标题

**响应**：
```json
{
  "success": true,
  "data": {
    "job_id": "uuid",
    "message": "Document upload started"
  }
}
```

#### 批量上传

```
POST /api/v1/documents/batch
Content-Type: multipart/form-data
```

**请求**：
- `files[]`: 多个文档文件

**响应**：
```json
{
  "success": true,
  "data": {
    "job_ids": ["uuid1", "uuid2"],
    "total": 2
  }
}
```

#### 列表文档

```
GET /api/v1/documents?limit=50&offset=0&sort_by=created_at&order=desc
```

**响应**：
```json
{
  "success": true,
  "data": {
    "documents": [...],
    "total": 100,
    "limit": 50,
    "offset": 0
  }
}
```

#### 删除文档

```
DELETE /api/v1/documents/{id}?hard=false
```

**参数**：
- `hard`: 是否硬删除（默认 false，软删除）

#### 批量删除

```
POST /api/v1/documents/batch-delete
Content-Type: application/json
```

**请求**：
```json
{
  "document_ids": ["id1", "id2", "id3"]
}
```

**响应**：
```json
{
  "success": true,
  "data": {
    "deleted": ["id1", "id2"],
    "failed": ["id3"],
    "errors": {"id3": "document not found"},
    "total": 3,
    "success": 2,
    "failed_count": 1
  }
}
```

### 任务管理

#### 查询任务状态

```
GET /api/v1/jobs/{id}
```

**响应**：
```json
{
  "success": true,
  "data": {
    "id": "uuid",
    "status": "processing",
    "progress": 50,
    "current_stage": "embedding",
    "stage_message": "Generating embeddings for 100 chunks",
    "progress_history": [...],
    "created_at": "2026-01-27T10:00:00Z",
    "updated_at": "2026-01-27T10:05:00Z"
  }
}
```

### 查询服务

#### RAG 查询

```
POST /api/v1/query
Content-Type: application/json
```

**请求**：
```json
{
  "query": "用户问题",
  "top_k": 5,
  "enable_rerank": true,
  "enable_hybrid": true
}
```

**响应**：
```json
{
  "success": true,
  "data": {
    "answer": "生成的回答...",
    "citations": [
      {
        "id": 1,
        "source": "document.md",
        "section": "Chapter1/Section2",
        "preview": "相关内容预览..."
      }
    ],
    "tokens_used": 1500,
    "query_used": "实际使用的查询（可能被重写）"
  }
}
```

#### 对话查询

```
POST /api/v1/conversations
Content-Type: application/json
```

**请求**：
```json
{
  "conversation_id": "uuid",
  "message": "用户消息",
  "top_k": 5
}
```

**响应**：
```json
{
  "success": true,
  "data": {
    "conversation_id": "uuid",
    "message_id": "uuid",
    "answer": "生成的回答...",
    "citations": [...]
  }
}
```

### 文档版本

#### 列出版本

```
GET /api/v1/documents/{id}/versions
```

#### 恢复版本

```
POST /api/v1/documents/{id}/restore-version
Content-Type: application/json
```

**请求**：
```json
{
  "version": 2
}
```

## 中间件

### 1. 请求日志

**功能**：记录所有请求和响应

**实现**：`internal/api/middleware.go`

**日志内容**：
- 请求方法、路径、参数
- 响应状态码、耗时
- 请求ID

### 2. 请求ID

**功能**：为每个请求生成唯一ID

**实现**：`RequestID()` 中间件

**Header**：
- `X-Request-ID`: 请求ID（客户端可传入）

### 3. 参数验证

**功能**：验证查询参数和请求体

**实现**：`ValidateQueryParams()`, `ValidateContentType()`

**验证内容**：
- 分页参数（limit, offset）
- 排序参数（sort_by, order）
- Content-Type

### 4. 限流

**功能**：防止API滥用

**实现**：`RateLimit()` 中间件

**配置**：
```yaml
server:
  rate_limit:
    enabled: true
    rate: 100        # 每秒请求数
    burst: 200       # 突发请求数
    window: 1s       # 时间窗口
```

**响应**：
- `429 Too Many Requests`
- `Retry-After` header

## 错误处理

### 统一错误格式

```json
{
  "success": false,
  "error": {
    "code": "INVALID_REQUEST",
    "message": "Invalid request parameters",
    "details": {
      "field": "limit",
      "reason": "must be between 1 and 1000"
    },
    "request_id": "uuid",
    "timestamp": "2026-01-27T10:00:00Z"
  }
}
```

### 错误代码

- `INVALID_REQUEST`: 无效请求
- `NOT_FOUND`: 资源不存在
- `INTERNAL_ERROR`: 内部错误
- `TIMEOUT`: 请求超时
- `RATE_LIMIT`: 限流
- `SERVICE_UNAVAILABLE`: 服务不可用

### 增强错误信息

```go
// 可重试错误
WriteRetryableError(w, code, message, details)

// 不可重试错误
WriteNonRetryableError(w, code, message, details)

// 带上下文的错误
WriteErrorWithContext(ctx, w, code, message, details)
```

## 响应格式

### 成功响应

```json
{
  "success": true,
  "data": { ... }
}
```

### 错误响应

```json
{
  "success": false,
  "error": {
    "code": "...",
    "message": "...",
    "details": { ... },
    "request_id": "...",
    "timestamp": "..."
  }
}
```

## 路由配置

**实现**：`internal/api/router.go`

**框架**：使用 `go-chi/chi` 路由库

**路由组织**：
```go
r.Route("/api/v1", func(r chi.Router) {
    r.Get("/health", handler.Health)
    r.Route("/documents", func(r chi.Router) {
        r.Post("/", handler.UploadDocument)
        r.Get("/", handler.ListDocuments)
        r.Delete("/{id}", handler.DeleteDocument)
        r.Post("/batch", handler.UploadDocumentsBatch)
        r.Post("/batch-delete", handler.DeleteDocumentsBatch)
    })
    r.Route("/jobs", func(r chi.Router) {
        r.Get("/{id}", handler.GetJob)
    })
    r.Post("/query", handler.Query)
    r.Route("/conversations", func(r chi.Router) {
        r.Post("/", handler.CreateConversation)
        r.Get("/{id}", handler.GetConversation)
    })
})
```

## 设计决策

### 1. RESTful 设计

- **原则**：遵循 REST 规范
- **优势**：易于理解和使用
- **版本**：URL 中包含版本号 `/api/v1`

### 2. 统一响应格式

- **格式**：`{success, data/error}`
- **优势**：客户端易于处理
- **一致性**：所有端点使用相同格式

### 3. 异步处理

- **策略**：耗时操作返回 job_id
- **优势**：避免长时间阻塞
- **查询**：通过 `/jobs/{id}` 查询状态

### 4. 中间件链

- **顺序**：日志 → 请求ID → 验证 → 限流 → 处理
- **优势**：关注点分离，易于维护

## 依赖关系

- **输入**：HTTP 请求
- **依赖**：所有内部模块（ingestion, chunking, embedding, index, retrieval, prompt, generation, job, conversation）
- **输出**：HTTP 响应

## 测试

运行测试：

```bash
go test ./internal/api/... -v
```

测试覆盖：
- 端点功能
- 参数验证
- 错误处理
- 中间件
- 批量操作

## 监控指标

- `http_requests_total`：HTTP 请求总数
- `http_request_duration_seconds`：请求耗时
- `http_errors_total`：错误数
- `rate_limit_hits_total`：限流命中数

