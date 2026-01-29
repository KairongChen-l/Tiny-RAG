# 检索模块 (Retrieval)

## 模块职责

检索模块负责根据用户查询从向量存储中检索相关的文档块，支持多种检索策略（向量检索、BM25、混合检索）和结果重排序（Reranking）。

## 核心功能

- **向量检索**：基于语义相似度的向量搜索
- **BM25 检索**：基于关键词的全文检索
- **混合检索**：结合向量和 BM25 的混合策略
- **查询重写**：智能查询重写和扩展
- **结果重排序**：使用 Reranker 提升检索质量
- **元数据过滤**：支持基于元数据的过滤

## 核心接口

### Retriever 接口

```go
type Retriever interface {
    Retrieve(ctx context.Context, query string, opts RetrieveOptions) (*RetrievalResult, error)
}
```

### Reranker 接口

```go
type Reranker interface {
    Rerank(ctx context.Context, query string, chunks []RetrievedChunk) ([]RetrievedChunk, error)
}
```

## 检索策略

### 1. 向量检索 (Vector Retrieval)

**实现**：`internal/retrieval/vector.go`

**特点**：
- 基于语义相似度
- 使用余弦相似度计算
- 支持元数据过滤

**流程**：
1. 将查询文本转换为向量
2. 在向量存储中搜索相似向量
3. 返回 top-k 结果

### 2. BM25 检索

**实现**：`internal/retrieval/bm25.go`

**特点**：
- 基于关键词匹配
- 使用标准 BM25 算法
- 支持词干提取（Snowball）

**算法**：
- TF (Term Frequency)：词频
- IDF (Inverse Document Frequency)：逆文档频率
- 归一化：文档长度归一化

### 3. 混合检索 (Hybrid Retrieval)

**实现**：`internal/retrieval/hybrid.go`

**特点**：
- 结合向量检索和 BM25
- 支持多种融合策略

**融合策略**：

1. **RRF (Reciprocal Rank Fusion)** - 默认
   ```go
   score = 1 / (k + rank)
   final_score = sum(rrf_scores)
   ```

2. **Average**
   ```go
   final_score = (vector_score + bm25_score) / 2
   ```

3. **Max**
   ```go
   final_score = max(vector_score, bm25_score)
   ```

## 查询重写

### 简单重写器 (SimpleQueryRewriter)

**实现**：`internal/retrieval/rewriter.go`

**功能**：
- 去除停用词
- 大小写规范化
- 去除标点符号

### LLM 重写器 (LLMQueryRewriter)

**实现**：`internal/retrieval/llm_rewriter.go`

**功能**：
- 使用 LLM 进行智能重写
- 查询扩展（同义词、相关概念）
- 多查询生成（Multi-Query）

## 结果重排序 (Reranking)

### Cohere Reranker

**实现**：`internal/retrieval/reranker/cohere.go`

**特点**：
- 使用 Cohere Rerank API
- 模型：`rerank-multilingual-v3.0`
- 支持批量 rerank

**流程**：
1. 向量检索获取候选结果（如 top-20）
2. 使用 Reranker 重新排序
3. 返回最终 top-k 结果（如 top-5）

## 数据结构

### RetrieveOptions

```go
type RetrieveOptions struct {
    TopK               int               // 最终返回数量（默认：5）
    CandidateK        int               // Rerank 前候选数量（默认：20）
    MinScore           float32           // 最小相似度阈值
    MetadataFilter     map[string]string // 元数据过滤
    EnableRerank       bool              // 启用 Reranking
    EnableQueryRewrite bool              // 启用查询重写
    EnableQueryExpand  bool              // 启用查询扩展
    MultiQueryCount    int               // 多查询数量（0=禁用）
}
```

### RetrievalResult

```go
type RetrievalResult struct {
    Chunks    []RetrievedChunk
    QueryUsed string  // 实际使用的查询（可能被重写）
}
```

### RetrievedChunk

```go
type RetrievedChunk struct {
    chunking.Chunk
    Score      float32  // 相似度分数
    CitationID int      // 引用编号（1-based）
}
```

## 使用示例

### 基础向量检索

```go
retriever := retrieval.NewVectorRetriever(store, embedder)

opts := retrieval.DefaultRetrieveOptions()
opts.TopK = 10
opts.MinScore = 0.7

result, err := retriever.Retrieve(ctx, "用户查询", opts)
if err != nil {
    return err
}

for _, chunk := range result.Chunks {
    fmt.Printf("Score: %.3f, Content: %s\n", chunk.Score, chunk.Content)
}
```

### 混合检索

```go
// 创建混合检索器
vectorRetriever := retrieval.NewVectorRetriever(store, embedder)
bm25Retriever := retrieval.NewBM25Retriever(index)
hybridRetriever := retrieval.NewHybridRetriever(vectorRetriever, bm25Retriever, "rrf", 60)

opts := retrieval.DefaultRetrieveOptions()
opts.TopK = 10

result, err := hybridRetriever.Retrieve(ctx, "查询", opts)
```

### 带 Reranking 的检索

```go
// 创建 Reranker
reranker := reranker.NewCohereReranker(cohereAPIKey, "rerank-multilingual-v3.0", 10)

// 创建带 Reranker 的检索器
retriever := retrieval.NewVectorRetriever(store, embedder)
retriever.SetReranker(reranker)

opts := retrieval.DefaultRetrieveOptions()
opts.TopK = 5
opts.CandidateK = 20  // Rerank 前获取 20 个候选
opts.EnableRerank = true

result, err := retriever.Retrieve(ctx, "查询", opts)
```

### 查询重写

```go
// 启用查询重写
opts := retrieval.DefaultRetrieveOptions()
opts.EnableQueryRewrite = true
opts.EnableQueryExpand = true
opts.MultiQueryCount = 3  // 生成 3 个查询变体

result, err := retriever.Retrieve(ctx, "原始查询", opts)
fmt.Printf("Used query: %s\n", result.QueryUsed)
```

## 设计决策

### 1. 混合检索策略

- **选择**：RRF (Reciprocal Rank Fusion)
- **理由**：结合语义和关键词，提升召回率
- **Trade-off**：计算成本略高，但检索质量显著提升

### 2. Reranking 两阶段检索

- **策略**：先检索更多候选，再重排序
- **优势**：平衡速度和精度
- **参数**：`CandidateK` (20) > `TopK` (5)

### 3. 查询重写

- **实现**：可选功能，默认关闭
- **优势**：提升模糊查询的检索质量
- **成本**：LLM 重写需要额外 API 调用

## 性能优化

### 1. 并行检索

- 向量检索和 BM25 检索并行执行
- 减少总体延迟

### 2. 批量 Reranking

- Reranker 支持批量处理
- 减少 API 调用次数

### 3. 缓存查询结果

- 相同查询可缓存结果
- 减少重复计算

## 配置示例

```yaml
retrieval:
  enable_hybrid: true
  fusion_method: "rrf"  # "rrf", "avg", or "max"
  rrf_k: 60
  enable_rerank: true
  cohere:
    api_key: "${COHERE_API_KEY}"
    model: "rerank-multilingual-v3.0"
    top_n: 10
  enable_query_rewrite: true
  enable_query_expand: true
  multi_query_count: 3
```

## 依赖关系

- **输入**：用户查询字符串
- **依赖**：`index.VectorStore`、`embedding.Embedder`
- **输出**：`RetrievalResult`（供 `prompt` 模块使用）

## 测试

运行测试：

```bash
go test ./internal/retrieval/... -v
```

测试覆盖：
- 向量检索
- BM25 检索
- 混合检索
- Reranking
- 查询重写
- 元数据过滤

## 监控指标

- `retrieval_requests_total`：检索请求总数
- `retrieval_duration_seconds`：检索耗时
- `retrieval_results_count`：返回结果数量
- `retrieval_errors_total`：错误数
- `retrieval_rerank_requests_total`：Rerank 请求数

