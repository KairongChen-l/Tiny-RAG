# 提示词构建模块 (Prompt)

## 模块职责

提示词构建模块负责将用户查询和检索到的文档块组装成适合 LLM 处理的提示词，包括上下文组装、引用格式化和 Token 截断。

## 核心功能

- **上下文组装**：将检索到的 chunks 组装成上下文
- **引用格式化**：自动生成 `[citation:x]` 格式的引用
- **Token 截断**：智能截断以符合 LLM 上下文窗口限制
- **对话历史支持**：支持多轮对话的上下文构建
- **模板系统**：可配置的系统提示词模板

## 核心接口

### PromptBuilder 接口

```go
type PromptBuilder interface {
    Build(ctx context.Context, query string, chunks []retrieval.RetrievedChunk, opts PromptOptions) (*Prompt, error)
    BuildWithHistory(ctx context.Context, query string, chunks []retrieval.RetrievedChunk, history string, opts PromptOptions) (*Prompt, error)
}
```

## 数据结构

### Prompt

```go
type Prompt struct {
    SystemMessage string         // 系统消息内容
    UserMessage   string         // 用户消息内容
    TokenCount    int            // 估算的 token 数量
    Citations     []CitationInfo // 引用元数据
}
```

### CitationInfo

```go
type CitationInfo struct {
    ID          int    // 引用编号（1-based）
    DocumentID  string // 源文档ID
    Source      string // 源文件路径
    SectionPath string // 文档中的章节路径
    Preview     string // 内容预览
}
```

### PromptOptions

```go
type PromptOptions struct {
    MaxContextTokens int    // 最大上下文 token 数（默认：3000）
    IncludeCitations bool   // 包含引用说明（默认：true）
    SystemPrompt     string // 自定义系统提示词（可选）
}
```

## 核心功能实现

### 1. 上下文组装

**实现**：`internal/prompt/builder.go`

**流程**：
1. 按引用顺序添加 chunks
2. 每个 chunk 添加引用标记 `[citation:x]`
3. 包含 chunk 的元数据（来源、章节路径）
4. 智能截断以符合 token 限制

**格式示例**：
```
Context:
[citation:1] Source: document.md, Section: Chapter1/Section2
This is the first chunk content...

[citation:2] Source: document.md, Section: Chapter1/Section3
This is the second chunk content...
```

### 2. Token 截断

**实现**：`internal/prompt/truncator.go`

**策略**：
- 使用 tiktoken 精确计算 token 数
- 按引用顺序添加 chunks
- 超过限制时停止添加
- 保证至少包含一个 chunk

**算法**：
```go
1. 计算系统提示词和用户查询的 token 数
2. 计算可用上下文 token 数 = MaxContextTokens - 系统提示词 - 查询
3. 按顺序添加 chunks，累计 token 数
4. 超过限制时停止，返回已添加的 chunks
```

### 3. 引用格式化

**实现**：`internal/prompt/citation.go`

**格式**：
- 引用标记：`[citation:x]`（x 为 1-based 编号）
- 支持多个引用：`[citation:1][citation:2]`
- 自动解析 LLM 响应中的引用

**系统提示词**：
```
You are a helpful assistant. Answer the question based on the provided context.
Use the format [citation:x] to cite sources, where x is the context number.
If multiple contexts support a statement, cite all: [citation:1][citation:2].
If the context doesn't contain relevant information, say so.
Be concise and accurate.
```

### 4. 模板系统

**实现**：`internal/prompt/template.go`

**默认模板**：
- 系统提示词：包含引用格式说明
- 上下文格式：包含来源和章节信息
- 用户消息：查询 + 上下文

**自定义模板**：
- 支持通过 `PromptOptions.SystemPrompt` 自定义
- 保持引用格式一致性

## 使用示例

### 基础使用

```go
builder := prompt.NewDefaultPromptBuilder(tokenizer)

opts := prompt.DefaultPromptOptions()
opts.MaxContextTokens = 3000

prompt, err := builder.Build(ctx, "用户查询", chunks, opts)
if err != nil {
    return err
}

fmt.Printf("Token count: %d\n", prompt.TokenCount)
fmt.Printf("Citations: %d\n", len(prompt.Citations))

// 发送给 LLM
llmResponse, err := llm.Generate(ctx, prompt)
```

### 带对话历史

```go
history := "User: 之前的问题\nAssistant: 之前的回答"

prompt, err := builder.BuildWithHistory(ctx, "新问题", chunks, history, opts)
```

### 自定义系统提示词

```go
opts := prompt.DefaultPromptOptions()
opts.SystemPrompt = "You are a technical expert. Answer with detailed explanations."

prompt, err := builder.Build(ctx, "查询", chunks, opts)
```

## 设计决策

### 1. 引用格式

- **选择**：`[citation:x]` 格式
- **理由**：业界通用，易于解析
- **要求**：需要 LLM 遵循指令格式

### 2. Token 截断策略

- **策略**：按引用顺序添加，超限停止
- **优势**：保证引用编号连续
- **Trade-off**：可能丢失部分相关信息

### 3. 上下文组装

- **格式**：包含来源、章节路径、内容
- **优势**：便于用户溯源
- **成本**：略微增加 token 消耗

## 性能优化

### 1. Token 计算缓存

- 相同文本的 token 计算可缓存
- 减少重复计算

### 2. 批量处理

- 支持批量构建提示词
- 提高吞吐量

## 错误处理

- **Token 计算错误**：fallback 到字符数估算
- **截断失败**：至少返回一个 chunk
- **引用解析错误**：记录警告，继续处理

## 依赖关系

- **输入**：`retrieval.RetrievedChunk`（来自 `retrieval` 模块）
- **依赖**：`pkg/tokenizer`（token 计算）
- **输出**：`Prompt`（供 `generation` 模块使用）

## 测试

运行测试：

```bash
go test ./internal/prompt/... -v
```

测试覆盖：
- 上下文组装
- Token 截断
- 引用格式化
- 对话历史
- 错误处理

