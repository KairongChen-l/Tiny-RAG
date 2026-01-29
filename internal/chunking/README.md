# 文档分块模块 (Chunking)

## 模块职责

文档分块模块负责将完整的文档切分为适合向量化和检索的文本块（chunks），同时保持语义完整性和上下文信息。

## 核心功能

- **结构感知分块**：尊重文档结构（标题、段落边界）
- **语义分块**：基于 embedding 相似度的语义边界检测
- **重叠机制**：相邻块之间保留重叠，避免信息切断
- **可配置策略**：支持多种分块策略和参数配置

## 核心接口

### Chunker 接口

```go
type Chunker interface {
    Split(doc *ingestion.Document) ([]Chunk, error)
}
```

## 数据结构

### Chunk

```go
type Chunk struct {
    ID          string            // 唯一标识符
    DocumentID  string            // 所属文档ID
    Content     string            // 块内容
    SectionPath string            // 章节路径（如 "Chapter1/Section2"）
    Index       int               // 在文档中的索引
    Metadata    map[string]string // 元数据
}
```

### ChunkerConfig

```go
type ChunkerConfig struct {
    MaxChunkSize   int  // 最大块大小（字符数，默认：1000）
    MinChunkSize   int  // 最小块大小（字符数，默认：100）
    Overlap        int  // 块间重叠（字符数，默认：100）
    RespectBounds  bool // 尊重段落/标题边界（默认：true）
    SplitByHeading bool // 按标题分割（默认：true）
}
```

## 分块策略

### 1. 结构感知分块 (Semantic Chunker)

**实现**：`internal/chunking/semantic.go`

**特点**：
- 优先按标题层级分割
- 尊重段落边界
- 保持最小/最大块大小约束
- 相邻块保留重叠

**算法流程**：
1. 遍历文档的层次化结构
2. 按标题层级识别分割点
3. 在段落边界处创建块
4. 应用大小约束和重叠策略

### 2. 语义分块 (Embedding-based Semantic Chunking)

**实现**：`internal/chunking/embedding_semantic.go`

**特点**：
- 使用 embedding 相似度检测语义边界
- 当相邻句子相似度低于阈值时创建新块
- 自动 fallback 到结构感知分块

**算法流程**：
1. 将文档分割为句子
2. 对每个句子生成 embedding
3. 计算相邻句子的余弦相似度
4. 相似度低于阈值时创建块边界
5. 应用大小约束

## 使用示例

### 基础使用

```go
// 创建配置
config := chunking.DefaultChunkerConfig()
config.MaxChunkSize = 1000
config.Overlap = 100

// 创建分块器
chunker := chunking.NewSemanticChunker(config)

// 分块
chunks, err := chunker.Split(doc)
if err != nil {
    return err
}

// 使用块
for _, chunk := range chunks {
    fmt.Printf("Chunk %d: %s\n", chunk.Index, chunk.Content[:50])
    fmt.Printf("Section: %s\n", chunk.SectionPath)
}
```

### 语义分块

```go
// 创建语义分块器（需要 embedder）
config := chunking.DefaultChunkerConfig()
chunker := chunking.NewEmbeddingSemanticChunker(config, embedder, 0.7)

chunks, err := chunker.Split(doc)
```

## 设计决策

### 1. 结构感知 vs 固定大小

- **选择**：结构感知分块
- **理由**：保持语义完整性，提高检索质量
- **Trade-off**：实现复杂度更高，但检索效果更好

### 2. 重叠机制

- **实现**：相邻块保留 ~100 字符重叠
- **优势**：避免在句子中间切断，保持上下文
- **成本**：略微增加存储和索引成本

### 3. SectionPath 追踪

- **实现**：记录块所属的标题路径
- **优势**：支持精确引用定位，便于用户溯源
- **格式**：`"Chapter1/Section2/Subsection1"`

## 配置参数说明

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `MaxChunkSize` | 1000 | 最大块大小（字符数），超过会强制分割 |
| `MinChunkSize` | 100 | 最小块大小，避免过小的块 |
| `Overlap` | 100 | 块间重叠字符数，保持上下文连续性 |
| `RespectBounds` | true | 是否尊重段落/标题边界 |
| `SplitByHeading` | true | 是否按标题层级分割 |

## 性能考虑

- **批量处理**：支持批量文档分块
- **内存效率**：流式处理大文档，避免一次性加载
- **并发安全**：分块器是无状态的，可并发使用

## 扩展新策略

要添加新的分块策略：

1. 实现 `Chunker` 接口
2. 定义配置结构（如需要）
3. 实现 `Split` 方法

示例：

```go
type CustomChunker struct {
    config ChunkerConfig
}

func (c *CustomChunker) Split(doc *ingestion.Document) ([]Chunk, error) {
    // 实现自定义分块逻辑
}
```

## 依赖关系

- **输入**：`ingestion.Document`
- **输出**：`[]Chunk` 供 `embedding` 和 `index` 模块使用

## 测试

运行测试：

```bash
go test ./internal/chunking/... -v
```

测试覆盖：
- 结构感知分块
- 大小约束
- 重叠机制
- 边界处理
- 错误场景

