# LLM 生成模块 (Generation)

## 模块职责

LLM 生成模块负责与各种大语言模型（LLM）Provider 交互，生成基于上下文的回答。支持多种 Provider（OpenAI、Anthropic、Ollama、Kimi），并提供流式响应和错误处理。

## 核心功能

- **多 Provider 支持**：OpenAI、Anthropic、Ollama、Kimi
- **流式响应**：支持流式生成，实时返回结果
- **错误处理**：智能错误分类、重试机制、熔断器
- **Token 管理**：精确的 token 计数和限制
- **超时控制**：可配置的超时时间

## 核心接口

### LLM 接口

```go
type LLM interface {
    Generate(ctx context.Context, p *prompt.Prompt) (*Response, error)
    Name() string
}
```

### LLMRegistry

使用注册表模式管理多个 provider：

```go
registry := generation.NewLLMRegistry()
registry.Register("openai", openaiLLM)
registry.Register("ollama", ollamaLLM)
registry.SetDefault("openai")
```

## 支持的 Provider

### 1. OpenAI

**实现**：`internal/generation/openai/client.go`

**特点**：
- 模型：`gpt-4`, `gpt-3.5-turbo` 等
- 支持流式响应
- 精确的 token 计数
- 自动重试和错误处理

**配置**：
```yaml
generation:
  provider: "openai"
  openai:
    api_key: "${OPENAI_API_KEY}"
    model: "gpt-4"
    max_tokens: 2048
    temperature: 0.7
    timeout: 60s
```

### 2. Anthropic Claude

**实现**：`internal/generation/anthropic/client.go`

**特点**：
- 模型：`claude-3-opus`, `claude-3-sonnet` 等
- 支持长上下文
- 流式响应

### 3. Ollama

**实现**：`internal/generation/ollama/client.go`

**特点**：
- 本地部署，无需 API Key
- 模型：`llama3.2`, `qwen2.5` 等
- 支持自定义 base URL

**配置**：
```yaml
generation:
  provider: "ollama"
  ollama:
    base_url: "http://localhost:11434"
    model: "llama3.2"
    max_tokens: 2048
    temperature: 0.7
```

### 4. Kimi

**实现**：`internal/generation/kimi/client.go`

**特点**：
- 支持超长上下文（200K tokens）
- 中文优化
- 流式响应

## 数据结构

### Response

```go
type Response struct {
    Answer       string // 生成的回答
    TokensUsed   int    // 使用的 token 数
    FinishReason string // 完成原因（stop, length, etc.）
    Citations    []int  // 解析出的引用编号
}
```

### LLMConfig

```go
type LLMConfig struct {
    Model       string
    MaxTokens   int
    Temperature float32
    Timeout     time.Duration
}
```

## 使用示例

### 基础使用

```go
// 获取 LLM
llm, err := registry.GetDefault()
if err != nil {
    return err
}

// 生成回答
response, err := llm.Generate(ctx, prompt)
if err != nil {
    return err
}

fmt.Printf("Answer: %s\n", response.Answer)
fmt.Printf("Tokens used: %d\n", response.TokensUsed)
fmt.Printf("Citations: %v\n", response.Citations)
```

### 流式响应

```go
// 流式生成（如果 provider 支持）
stream, err := llm.GenerateStream(ctx, prompt)
if err != nil {
    return err
}

for chunk := range stream {
    fmt.Print(chunk.Text)
}
```

### 错误处理

```go
response, err := llm.Generate(ctx, prompt)
if err != nil {
    // 检查错误类型
    if errors.Is(err, ErrTimeout) {
        // 超时错误
    } else if errors.Is(err, ErrRateLimit) {
        // 限流错误，可重试
    } else if errors.Is(err, ErrAuth) {
        // 认证错误，不可重试
    }
}
```

## 设计决策

### 1. 注册表模式 (Registry Pattern)

- **优势**：易于切换 provider，支持运行时注册
- **实现**：使用 `sync.RWMutex` 保证并发安全

### 2. 错误处理策略

- **重试机制**：网络错误、超时、限流自动重试
- **熔断器**：防止级联失败
- **错误分类**：区分可重试和不可重试错误

### 3. 超时控制

- **实现**：使用 `context.WithTimeout`
- **默认**：60 秒
- **可配置**：通过 `LLMConfig.Timeout`

### 4. Token 管理

- **计数**：精确统计输入和输出 token
- **限制**：`MaxTokens` 限制输出长度
- **成本**：可用于成本计算和监控

## 错误处理

### 错误类型

- **网络错误**：自动重试（可配置次数）
- **超时错误**：返回明确的超时信息
- **限流错误**：自动重试，带退避
- **认证错误**：不重试，立即返回
- **无效请求**：不重试，返回详细错误

### 重试机制

- **指数退避**：重试间隔逐渐增加
- **最大重试次数**：可配置（默认 3 次）
- **非重试错误**：认证错误、无效请求不重试

### 熔断器

- **状态**：关闭、开启、半开
- **阈值**：失败次数达到阈值时开启
- **恢复**：超时后进入半开状态，成功则关闭

## 性能优化

### 1. 流式响应

- 实时返回生成结果
- 减少用户等待时间
- 支持取消操作

### 2. 并发控制

- LLM 实现是线程安全的
- 支持并发调用
- 注意 provider 的速率限制

### 3. 连接池

- 复用 HTTP 连接
- 减少连接建立开销

## 扩展新 Provider

要添加新的 LLM provider：

1. 实现 `LLM` 接口
2. 实现 `Generate` 方法
3. 注册到 `LLMRegistry`

示例：

```go
type CustomLLM struct {
    config CustomConfig
}

func (l *CustomLLM) Generate(ctx context.Context, p *prompt.Prompt) (*Response, error) {
    // 实现生成逻辑
}

func (l *CustomLLM) Name() string {
    return "custom"
}

// 注册
registry.Register("custom", &CustomLLM{})
```

## 依赖关系

- **输入**：`prompt.Prompt`（来自 `prompt` 模块）
- **输出**：`Response`（供 API 层返回）
- **外部依赖**：OpenAI API、Anthropic API、Ollama 服务等

## 测试

运行测试：

```bash
go test ./internal/generation/... -v
```

测试覆盖：
- 基础生成
- 流式响应
- 错误处理
- 重试机制
- 超时控制

## 监控指标

- `llm_requests_total`：请求总数
- `llm_duration_seconds`：请求耗时
- `llm_tokens_total`：使用的 token 总数
- `llm_errors_total`：错误数
- `llm_circuit_breaker_state`：熔断器状态

