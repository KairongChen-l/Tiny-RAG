# 向量嵌入模块 (Embedding)

## 模块职责

向量嵌入模块负责将文本转换为高维向量表示，用于后续的向量相似度检索。支持多种 embedding provider，并提供缓存机制以优化成本和性能。

## 核心功能

- **多 Provider 支持**：OpenAI、Ollama 等
- **批量处理**：支持批量 embedding 以提高效率
- **缓存机制**：内存缓存和 Redis 缓存，减少重复计算
- **错误重试**：智能错误分类和自动重试
- **成本优化**：通过缓存显著降低 API 调用成本

## 核心接口

### Embedder 接口

```go
type Embedder interface {
    Embed(ctx context.Context, text string) ([]float32, error)
    EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
    Dimensions() int
    Name() string
}
```

### EmbedderRegistry

使用注册表模式管理多个 provider：

```go
registry := NewEmbedderRegistry()
registry.Register("openai", openaiEmbedder)
registry.Register("ollama", ollamaEmbedder)
registry.SetDefault("openai")
```

## 支持的 Provider

### 1. OpenAI Embedding

**实现**：`internal/embedding/openai.go`

**特点**：
- 模型：`text-embedding-3-small` (默认)
- 维度：1536
- 支持批量处理
- 自动重试和错误处理

**配置**：
```yaml
embedding:
  provider: "openai"
  openai:
    api_key: "${OPENAI_API_KEY}"
    model: "text-embedding-3-small"
    dimensions: 1536
    batch_size: 100
```

### 2. Ollama Embedding

**实现**：`internal/embedding/ollama.go`

**特点**：
- 本地部署，无需 API Key
- 模型：`nomic-embed-text` (默认)
- 维度：768
- 支持自定义 base URL

**配置**：
```yaml
embedding:
  provider: "ollama"
  ollama:
    base_url: "http://localhost:11434"
    model: "nomic-embed-text"
    dimensions: 768
    batch_size: 50
```

## 缓存机制

### 内存缓存

**实现**：`internal/embedding/cache_memory.go`

**特点**：
- 进程内缓存，速度快
- 支持 TTL 和最大条目数限制
- 自动清理过期条目

**使用**：
```go
cache := NewMemoryCache(24*time.Hour, 10000)
cachedEmbedder := NewCachedEmbedder(embedder, cache)
```

### Redis 缓存

**实现**：`internal/embedding/cache_redis.go`

**特点**：
- 分布式缓存，多进程共享
- 支持 TTL
- 缓存键格式：`embed:{provider}:{model}:{hash(text)}`

**使用**：
```go
cache := NewRedisCache(redisClient, 24*time.Hour)
cachedEmbedder := NewCachedEmbedder(embedder, cache)
```

## 使用示例

### 基础使用

```go
// 创建 embedder
embedder, err := registry.GetDefault()
if err != nil {
    return err
}

// 单个文本 embedding
vector, err := embedder.Embed(ctx, "Hello, world!")
if err != nil {
    return err
}

fmt.Printf("Dimensions: %d\n", embedder.Dimensions())
fmt.Printf("Vector length: %d\n", len(vector))
```

### 批量处理

```go
// 批量 embedding
texts := []string{"text1", "text2", "text3"}
vectors, err := embedder.EmbedBatch(ctx, texts)
if err != nil {
    return err
}

// 使用向量
for i, vector := range vectors {
    fmt.Printf("Text %d: %d dimensions\n", i, len(vector))
}
```

### 使用缓存

```go
// 创建缓存
cache := NewMemoryCache(24*time.Hour, 10000)

// 包装 embedder
cachedEmbedder := NewCachedEmbedder(embedder, cache)

// 第一次调用：从 API 获取
vector1, _ := cachedEmbedder.Embed(ctx, "Hello")

// 第二次调用：从缓存获取（更快，无成本）
vector2, _ := cachedEmbedder.Embed(ctx, "Hello")
```

## 设计决策

### 1. 注册表模式 (Registry Pattern)

- **优势**：易于切换 provider，支持运行时注册
- **实现**：使用 `sync.RWMutex` 保证并发安全

### 2. 批量处理优化

- **实现**：`EmbedBatch` 方法支持批量 API 调用
- **优势**：减少网络往返，提高吞吐量
- **注意**：需要 provider 支持批量 API

### 3. 缓存策略

- **键格式**：`embed:{provider}:{model}:{hash(text)}`
- **TTL**：默认 24 小时，可配置
- **成本**：缓存命中可减少 30-50% API 调用

### 4. 错误处理

- **重试机制**：网络错误、超时自动重试
- **错误分类**：区分可重试和不可重试错误
- **详细错误**：包含 provider、模型、URL 等信息

## 性能优化

### 1. 批量处理

- 将多个文本合并为一次 API 调用
- 减少网络延迟
- 提高吞吐量

### 2. 缓存命中

- 相同文本直接返回缓存结果
- 零延迟，零成本
- 适合重复内容场景

### 3. 并发控制

- Embedder 实现是线程安全的
- 支持并发调用
- 注意 provider 的速率限制

## 错误处理

### 错误类型

- **网络错误**：自动重试（可配置次数）
- **认证错误**：不重试，立即返回
- **限流错误**：自动重试，带退避
- **超时错误**：可配置超时时间

### 错误信息格式

```
Ollama API error (model: nomic-embed-text, base_url: http://localhost:11434): connection refused
```

## 扩展新 Provider

要添加新的 embedding provider：

1. 实现 `Embedder` 接口
2. 实现 `Embed` 和 `EmbedBatch` 方法
3. 注册到 `EmbedderRegistry`

示例：

```go
type CustomEmbedder struct {
    config CustomConfig
}

func (e *CustomEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
    // 实现 embedding 逻辑
}

func (e *CustomEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
    // 实现批量 embedding
}

func (e *CustomEmbedder) Dimensions() int {
    return e.config.Dimensions
}

func (e *CustomEmbedder) Name() string {
    return "custom"
}

// 注册
registry.Register("custom", &CustomEmbedder{})
```

## 依赖关系

- **输入**：文本字符串（来自 `chunking` 模块）
- **输出**：向量数组（供 `index` 模块存储）
- **外部依赖**：OpenAI API、Ollama 服务

## 测试

运行测试：

```bash
go test ./internal/embedding/... -v
```

测试覆盖：
- 单个 embedding
- 批量 embedding
- 错误处理
- 缓存机制
- 并发安全

## 监控指标

- `embedding_requests_total`：总请求数
- `embedding_duration_seconds`：请求耗时
- `embedding_cache_hits_total`：缓存命中数
- `embedding_cache_misses_total`：缓存未命中数
- `embedding_errors_total`：错误数

