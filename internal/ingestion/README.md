# 文档摄取模块 (Ingestion)

## 模块职责

文档摄取模块负责从各种格式的文档中提取结构化内容，将原始文件转换为系统可处理的 `Document` 结构。

## 核心功能

- **多格式支持**：支持 Markdown、PDF、纯文本格式
- **结构化解析**：保留文档的层次结构（标题层级、段落等）
- **格式检测**：自动识别文档格式
- **元数据提取**：支持自定义元数据

## 核心接口

### Parser 接口

```go
type Parser interface {
    Parse(ctx context.Context, path string) (*Document, error)
    SupportedFormats() []DocumentFormat
}
```

### ParserRegistry

使用注册表模式管理多种解析器：

```go
registry := NewParserRegistry()
registry.Register(NewMarkdownParser())
registry.Register(NewTextParser())
registry.Register(NewPDFParser())
```

## 数据结构

### Document

```go
type Document struct {
    ID       string            // 唯一标识符
    Title    string            // 文档标题
    Source   string            // 原始文件路径
    Format   DocumentFormat    // 文档格式
    Sections []Section         // 层次化章节
    Metadata map[string]string // 自定义元数据
    Hash     string            // 内容哈希（用于增量检测）
}
```

### Section

```go
type Section struct {
    Title    string    // 章节标题
    Level    int       // 标题层级 (1=H1, 2=H2, ...)
    Content  string    // 章节内容
    Children []Section // 嵌套子章节
}
```

## 支持的格式

- **Markdown** (`FormatMarkdown`): 支持 `.md`, `.markdown` 文件
- **PDF** (`FormatPDF`): 支持 `.pdf` 文件
- **纯文本** (`FormatText`): 支持 `.txt`, `.text` 文件

## 使用示例

```go
// 创建解析器注册表
registry := NewParserRegistry()
registry.Register(NewMarkdownParser())

// 检测格式
format, err := DetectFormat("document.md")
if err != nil {
    return err
}

// 获取解析器
parser, err := registry.GetParser(format)
if err != nil {
    return err
}

// 解析文档
doc, err := parser.Parse(ctx, "document.md")
if err != nil {
    return err
}

// 使用文档
fmt.Printf("Title: %s\n", doc.Title)
fmt.Printf("Sections: %d\n", len(doc.Sections))
```

## 设计决策

### 1. 注册表模式 (Registry Pattern)

- **优势**：易于扩展新格式，支持运行时注册
- **实现**：使用 `sync.RWMutex` 保证并发安全

### 2. 层次化结构

- **优势**：保留文档语义结构，便于后续分块和引用
- **实现**：使用嵌套 `Section` 结构表示文档层级

### 3. 格式检测

- **实现**：基于文件扩展名自动检测
- **扩展性**：可通过注册表添加新格式

## 扩展新格式

要添加新的文档格式支持：

1. 实现 `Parser` 接口
2. 在 `SupportedFormats()` 中返回支持的格式
3. 注册到 `ParserRegistry`

示例：

```go
type CustomParser struct{}

func (p *CustomParser) Parse(ctx context.Context, path string) (*Document, error) {
    // 实现解析逻辑
}

func (p *CustomParser) SupportedFormats() []DocumentFormat {
    return []DocumentFormat{FormatCustom}
}

// 注册
registry.Register(&CustomParser{})
```

## 错误处理

- 文件不存在：返回明确的错误信息
- 格式不支持：返回格式错误
- 解析失败：返回详细的解析错误

## 依赖关系

- 无外部依赖（标准库）
- 为 `chunking` 模块提供输入

## 测试

运行测试：

```bash
go test ./internal/ingestion/... -v
```

测试覆盖：
- 格式检测
- 解析器注册和获取
- Markdown 解析
- 文本解析
- 错误处理

