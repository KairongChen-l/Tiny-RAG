# 向量存储模块 (Index)

## 模块职责

向量存储模块负责存储文档、chunks 和向量数据，提供向量相似度检索功能。支持多种存储后端（SQLite + sqlite-vec、Qdrant），并提供版本控制和软删除功能。

## 核心功能

- **向量存储**：存储文档 chunks 及其向量表示
- **相似度检索**：基于向量相似度的语义搜索
- **元数据过滤**：支持基于元数据的过滤查询
- **增量更新**：基于 hash 的增量检测和原子替换
- **版本控制**：文档版本快照和回滚
- **软删除**：支持软删除和恢复

## 核心接口

### VectorStore 接口

```go
type VectorStore interface {
    Store(ctx context.Context, chunks []chunking.ChunkWithVector) error
    Search(ctx context.Context, query []float32, opts SearchOptions) ([]SearchResult, error)
    DeleteByDocument(ctx context.Context, documentID string) error
    GetDocumentHash(ctx context.Context, documentID string) (string, error)
    ReplaceDocument(ctx context.Context, documentID string, chunks []chunking.ChunkWithVector) error
    ListDocuments(ctx context.Context, opts ListOptions) (*ListDocumentsResult, error)
    GetStats(ctx context.Context) (*Stats, error)
    Close() error
    
    // 版本控制（可选）
    StoreVersion(ctx context.Context, documentID string, changeNote string) error
    ListVersions(ctx context.Context, documentID string) ([]DocumentVersion, error)
    RestoreVersion(ctx context.Context, documentID string, version int) error
    
    // 软删除
    SoftDeleteDocument(ctx context.Context, documentID string) error
    RestoreDocument(ctx context.Context, documentID string) error
    HardDeleteDocument(ctx context.Context, documentID string) error
}
```

## 支持的存储后端

### 1. SQLite + sqlite-vec

**实现**：`internal/index/sqlite/store.go`

**特点**：
- 零依赖部署（需要编译 sqlite-vec 扩展）
- 单文件存储，易于备份
- 支持版本控制和软删除
- 适合中小规模数据（< 100万 chunks）

**表结构**：
- `documents`：文档元数据
- `chunks`：文本块数据
- `chunk_vectors`：向量数据（sqlite-vec）
- `document_versions`：版本快照
- `jobs`：任务状态

**配置**：
```yaml
database:
  provider: "sqlite"
  sqlite:
    path: "./data/rag.db"
```

### 2. Qdrant

**实现**：`internal/index/qdrant/store.go`

**特点**：
- 高性能向量检索
- 支持分布式部署
- 丰富的元数据过滤
- 无需 CGO 依赖
- 适合大规模数据（> 100万 chunks）

**配置**：
```yaml
database:
  provider: "qdrant"
  qdrant:
    url: "http://localhost:6333"
    collection: "rag_chunks"
```

## 数据结构

### SearchOptions

```go
type SearchOptions struct {
    TopK           int               // 返回结果数量
    MinScore       float32           // 最小相似度阈值
    MetadataFilter map[string]string // 元数据过滤
}
```

### SearchResult

```go
type SearchResult struct {
    Chunk    chunking.Chunk
    Score    float32  // 相似度分数
    Citation int      // 引用编号（1-based）
}
```

### ListOptions

```go
type ListOptions struct {
    Limit          int               // 分页限制
    Offset         int               // 分页偏移
    SortBy         string            // 排序字段
    Order          string            // 排序顺序
    IncludeDeleted bool              // 包含已删除文档
    SearchQuery    string            // 全文搜索
    FilterBy       map[string]string // 元数据过滤
}
```

## 使用示例

### 存储文档

```go
// 创建 chunks with vectors
chunksWithVectors := make([]chunking.ChunkWithVector, len(chunks))
for i, chunk := range chunks {
    vector, _ := embedder.Embed(ctx, chunk.Content)
    chunksWithVectors[i] = chunking.ChunkWithVector{
        Chunk: chunk,
        Vector: vector,
    }
}

// 存储
err := store.Store(ctx, chunksWithVectors)
```

### 向量搜索

```go
// 生成查询向量
queryVector, _ := embedder.Embed(ctx, "用户查询")

// 搜索
opts := SearchOptions{
    TopK:     10,
    MinScore: 0.7,
    MetadataFilter: map[string]string{
        "format": "markdown",
    },
}

results, err := store.Search(ctx, queryVector, opts)
for _, result := range results {
    fmt.Printf("Score: %.3f, Content: %s\n", result.Score, result.Chunk.Content)
}
```

### 列表文档

```go
opts := ListOptions{
    Limit:  50,
    Offset: 0,
    SortBy: "created_at",
    Order:  "desc",
}

result, err := store.ListDocuments(ctx, opts)
fmt.Printf("Total: %d, Returned: %d\n", result.Total, len(result.Documents))
```

### 增量更新

```go
// 检查文档 hash
oldHash, _ := store.GetDocumentHash(ctx, docID)
newHash := computeHash(doc)

if oldHash != newHash {
    // 原子替换
    err := store.ReplaceDocument(ctx, docID, newChunksWithVectors)
}
```

## 设计决策

### 1. 多后端支持

- **策略**：接口抽象 + 多实现
- **优势**：可根据场景选择合适后端
- **Trade-off**：需要维护多个实现

### 2. 增量更新机制

- **实现**：基于内容 hash 检测变更
- **优势**：避免重复处理，节省成本
- **原子性**：使用事务保证原子替换

### 3. 版本控制

- **实现**：快照机制，存储完整版本信息
- **优势**：支持回滚，历史追踪
- **限制**：SQLite 支持，Qdrant 不支持

### 4. 软删除

- **实现**：`deleted_at` 字段标记
- **优势**：数据可恢复，提高安全性
- **查询**：默认过滤已删除文档

## 性能优化

### 1. 批量操作

- `Store` 支持批量插入
- 使用事务保证原子性
- 减少数据库往返

### 2. 索引优化

- SQLite：在 `document_id`, `chunk_index` 上建立索引
- Qdrant：自动建立向量索引（HNSW）

### 3. 分页查询

- `ListDocuments` 支持分页
- 避免一次性加载大量数据
- 支持排序和过滤

## 错误处理

- **连接错误**：返回明确的错误信息
- **查询错误**：包含查询参数信息
- **版本控制错误**：区分不支持的操作

## 扩展新后端

要添加新的存储后端：

1. 实现 `VectorStore` 接口
2. 实现所有必需方法
3. 在配置中添加 provider 选项

示例：

```go
type CustomStore struct {
    // 自定义字段
}

func (s *CustomStore) Store(ctx context.Context, chunks []chunking.ChunkWithVector) error {
    // 实现存储逻辑
}

func (s *CustomStore) Search(ctx context.Context, query []float32, opts SearchOptions) ([]SearchResult, error) {
    // 实现搜索逻辑
}

// ... 实现其他方法
```

## 依赖关系

- **输入**：`chunking.ChunkWithVector`（来自 `chunking` 和 `embedding` 模块）
- **输出**：`SearchResult`（供 `retrieval` 模块使用）
- **外部依赖**：SQLite/sqlite-vec 或 Qdrant

## 测试

运行测试：

```bash
go test ./internal/index/... -v
```

测试覆盖：
- 存储和检索
- 增量更新
- 版本控制
- 软删除
- 分页和过滤
- 错误处理

## 监控指标

- `vector_store_operations_total`：操作总数
- `vector_store_duration_seconds`：操作耗时
- `vector_store_errors_total`：错误数
- `vector_store_documents_total`：文档总数
- `vector_store_chunks_total`：Chunks 总数

