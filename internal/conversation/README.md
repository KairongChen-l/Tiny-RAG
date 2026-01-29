# 对话管理模块 (Conversation)

## 模块职责

对话管理模块负责管理多轮对话的上下文和历史记录，支持对话的创建、消息添加、上下文窗口管理和对话摘要。

## 核心功能

- **多轮对话支持**：维护对话历史和上下文
- **消息管理**：添加、查询、过滤消息
- **上下文窗口**：智能截取最近 N 条消息
- **对话摘要**：生成对话摘要用于列表展示
- **引用追踪**：支持消息中的引用信息

## 核心数据结构

### Conversation

```go
type Conversation struct {
    ID        string    // 唯一标识符
    Title     string    // 对话标题
    Messages  []Message // 消息列表
    CreatedAt time.Time // 创建时间
    UpdatedAt time.Time // 更新时间
}
```

### Message

```go
type Message struct {
    ID        string     // 消息ID
    Role      Role       // 角色（user/assistant/system）
    Content   string     // 消息内容
    Citations []Citation // 引用信息（可选）
    CreatedAt time.Time  // 创建时间
}
```

### Role

```go
type Role string

const (
    RoleUser      Role = "user"
    RoleAssistant Role = "assistant"
    RoleSystem    Role = "system"
)
```

### Citation

```go
type Citation struct {
    ID      int    // 引用编号
    Source  string // 来源文档
    Section string // 章节路径
    Preview string // 内容预览
}
```

## 核心方法

### 创建对话

```go
conv := conversation.New("对话标题")
```

### 添加消息

```go
// 添加用户消息
msg := conv.AddMessage(conversation.RoleUser, "用户问题")

// 添加助手消息（带引用）
msg := conv.AddMessage(conversation.RoleAssistant, "助手回答")
msg.Citations = []conversation.Citation{
    {ID: 1, Source: "doc.md", Section: "Chapter1"},
}
```

### 获取上下文窗口

```go
// 获取最近 10 条消息
messages := conv.GetContextWindow(10)
```

### 转换为 LLM 格式

```go
// 转换为 prompt messages
promptMessages := conv.ToPromptMessages()
// 返回 []PromptMessage，可直接用于 LLM
```

### 生成摘要

```go
summary := conv.Summary()
// 返回 Summary 结构，包含 ID、标题、消息数、最后一条消息等
```

## 使用示例

### 创建和管理对话

```go
// 创建新对话
conv := conversation.New("技术咨询")

// 添加用户消息
conv.AddMessage(conversation.RoleUser, "什么是 RAG？")

// 添加助手消息
conv.AddMessage(conversation.RoleAssistant, "RAG 是检索增强生成...")

// 继续对话
conv.AddMessage(conversation.RoleUser, "它有什么优势？")
conv.AddMessage(conversation.RoleAssistant, "RAG 的优势包括...")

// 获取所有消息
messages := conv.GetMessages()
fmt.Printf("Total messages: %d\n", len(messages))
```

### 上下文窗口管理

```go
// 获取最近 5 条消息用于上下文
recentMessages := conv.GetContextWindow(5)

// 转换为 LLM 格式
promptMessages := conv.ToPromptMessages()

// 用于构建 prompt
for _, msg := range promptMessages {
    fmt.Printf("%s: %s\n", msg.Role, msg.Content)
}
```

### 对话摘要

```go
summary := conv.Summary()
fmt.Printf("ID: %s\n", summary.ID)
fmt.Printf("Title: %s\n", summary.Title)
fmt.Printf("Messages: %d\n", summary.MessageCount)
fmt.Printf("Last message: %s\n", summary.LastMessage)
```

## 设计决策

### 1. 内存存储

- **当前实现**：内存中的数据结构
- **扩展性**：可集成持久化存储（数据库）
- **优势**：简单高效，适合单进程场景

### 2. 上下文窗口

- **实现**：`GetContextWindow(n)` 返回最近 N 条消息
- **优势**：控制 token 消耗，保持相关性
- **策略**：从最新消息开始，向前取 N 条

### 3. 消息格式

- **角色**：user、assistant、system
- **引用**：可选字段，支持溯源
- **时间戳**：自动记录创建时间

## 持久化（扩展）

虽然当前实现是内存存储，但可以轻松扩展为持久化：

```go
// 扩展接口
type ConversationStore interface {
    Create(ctx context.Context, conv *Conversation) error
    Get(ctx context.Context, id string) (*Conversation, error)
    Update(ctx context.Context, conv *Conversation) error
    List(ctx context.Context, opts ListOptions) ([]Summary, error)
    Delete(ctx context.Context, id string) error
}
```

## 依赖关系

- **输入**：用户消息和助手消息
- **输出**：`PromptMessage`（供 `prompt` 模块使用）
- **无外部依赖**：纯内存数据结构

## 测试

运行测试：

```bash
go test ./internal/conversation/... -v
```

测试覆盖：
- 对话创建
- 消息添加
- 上下文窗口
- 对话摘要
- 格式转换

## 使用场景

### 1. 多轮对话

```go
// 第一轮
conv.AddMessage(conversation.RoleUser, "什么是 Go？")
conv.AddMessage(conversation.RoleAssistant, "Go 是...")

// 第二轮（带上下文）
conv.AddMessage(conversation.RoleUser, "它有什么特点？")
// 构建 prompt 时包含历史消息
```

### 2. 对话列表

```go
// 获取所有对话的摘要
conversations := []*conversation.Conversation{...}
summaries := make([]conversation.Summary, len(conversations))
for i, conv := range conversations {
    summaries[i] = conv.Summary()
}
```

### 3. 上下文管理

```go
// 根据 token 限制动态调整上下文窗口
maxTokens := 4000
usedTokens := estimateTokens(conv.GetMessages())
if usedTokens > maxTokens {
    // 只取最近的消息
    messages = conv.GetContextWindow(calculateWindowSize(maxTokens))
}
```

