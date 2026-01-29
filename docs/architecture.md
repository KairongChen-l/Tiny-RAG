# 系统架构文档

## 系统概览

本系统是一个生产级的 Go RAG (Retrieval-Augmented Generation) 知识库系统，采用模块化设计，支持多 Provider、异步处理、版本控制等高级功能。

## 整体架构图

```mermaid
graph TB
    subgraph "客户端层"
        Web[Web前端]
        API_Client[API客户端]
    end
    
    subgraph "HTTP API层"
        Router[路由层]
        Middleware[中间件<br/>日志/限流/验证]
        Handlers[请求处理器]
    end
    
    subgraph "核心业务模块"
        Ingestion[文档摄取<br/>ingestion]
        Chunking[文档分块<br/>chunking]
        Embedding[向量嵌入<br/>embedding]
        Index[向量存储<br/>index]
        Retrieval[检索模块<br/>retrieval]
        Prompt[提示词构建<br/>prompt]
        Generation[LLM生成<br/>generation]
        Conversation[对话管理<br/>conversation]
    end
    
    subgraph "异步任务模块"
        JobQueue[任务队列<br/>job]
        JobStore[任务存储]
    end
    
    subgraph "基础设施层"
        Cache[缓存<br/>pkg/cache]
        Retry[重试机制<br/>pkg/retry]
        CircuitBreaker[熔断器<br/>pkg/circuitbreaker]
        RateLimit[限流<br/>pkg/ratelimit]
        Metrics[监控指标<br/>metrics]
    end
    
    subgraph "存储层"
        VectorDB[(向量数据库<br/>SQLite/Qdrant)]
        JobDB[(任务数据库<br/>SQLite)]
        Redis[(Redis<br/>缓存/队列)]
    end
    
    subgraph "外部服务"
        OpenAI[OpenAI API]
        Ollama[Ollama服务]
        Cohere[Cohere API]
    end
    
    Web --> Router
    API_Client --> Router
    Router --> Middleware
    Middleware --> Handlers
    
    Handlers --> Ingestion
    Handlers --> Retrieval
    Handlers --> Conversation
    Handlers --> JobQueue
    
    Ingestion --> Chunking
    Chunking --> Embedding
    Embedding --> Index
    Index --> VectorDB
    
    Retrieval --> Index
    Retrieval --> Embedding
    Retrieval --> Prompt
    Prompt --> Generation
    Generation --> OpenAI
    Generation --> Ollama
    
    JobQueue --> JobStore
    JobStore --> JobDB
    
    Embedding --> Cache
    Cache --> Redis
    
    Generation --> Retry
    Generation --> CircuitBreaker
    Embedding --> Retry
    Embedding --> CircuitBreaker
    
    Router --> RateLimit
    Handlers --> Metrics
    
    Retrieval --> Cohere
```

## 数据流图

### 文档摄取流程

```mermaid
sequenceDiagram
    participant Client
    participant API
    participant JobQueue
    participant Ingestion
    participant Chunking
    participant Embedding
    participant Index
    
    Client->>API: POST /api/v1/documents
    API->>JobQueue: 提交任务
    API-->>Client: 返回 job_id
    
    JobQueue->>Ingestion: 解析文档
    Ingestion->>Ingestion: 提取结构化内容
    Ingestion->>Chunking: 文档分块
    Chunking->>Chunking: 结构感知分块
    Chunking->>Embedding: 生成向量
    Embedding->>Embedding: 批量embedding
    Embedding->>Index: 存储文档和向量
    Index->>Index: 原子替换（增量更新）
    Index-->>JobQueue: 完成
```

### 查询流程

```mermaid
sequenceDiagram
    participant Client
    participant API
    participant Retrieval
    participant Embedding
    participant Index
    participant Prompt
    participant Generation
    participant LLM
    
    Client->>API: POST /api/v1/query
    API->>Retrieval: 检索相关文档
    Retrieval->>Embedding: 查询向量化
    Embedding->>Index: 向量搜索
    Index-->>Retrieval: 返回相关chunks
    Retrieval->>Retrieval: 混合检索/Rerank
    Retrieval->>Prompt: 构建提示词
    Prompt->>Prompt: 组装上下文+引用
    Prompt->>Generation: 生成回答
    Generation->>LLM: 调用LLM API
    LLM-->>Generation: 返回回答
    Generation-->>API: 返回结果
    API-->>Client: 返回回答+引用
```

### 对话流程

```mermaid
sequenceDiagram
    participant Client
    participant API
    participant Conversation
    participant Retrieval
    participant Prompt
    participant Generation
    
    Client->>API: POST /api/v1/conversations
    API->>Conversation: 获取对话历史
    Conversation->>Conversation: 获取上下文窗口
    Conversation->>Retrieval: 检索相关文档
    Retrieval->>Prompt: 构建提示词（含历史）
    Prompt->>Generation: 生成回答
    Generation-->>API: 返回回答
    API->>Conversation: 保存消息
    API-->>Client: 返回回答
```

## 模块依赖关系

```mermaid
graph LR
    subgraph "核心模块"
        A[ingestion]
        B[chunking]
        C[embedding]
        D[index]
        E[retrieval]
        F[prompt]
        G[generation]
        H[conversation]
    end
    
    subgraph "基础设施"
        I[job]
        J[api]
        K[metrics]
    end
    
    A --> B
    B --> C
    C --> D
    E --> D
    E --> C
    E --> F
    F --> G
    H --> E
    H --> F
    J --> A
    J --> E
    J --> H
    J --> I
    I --> A
    I --> B
    I --> C
    I --> D
```

## 存储架构

### SQLite 存储结构

```mermaid
erDiagram
    documents ||--o{ chunks : contains
    documents ||--o{ document_versions : has
    chunks ||--|| chunk_vectors : has
    jobs ||--o{ job_progress : tracks
    
    documents {
        string id PK
        string title
        string source
        string format
        string hash
        datetime created_at
        datetime updated_at
        datetime deleted_at
    }
    
    chunks {
        string id PK
        string document_id FK
        string content
        string section_path
        int index
        json metadata
    }
    
    chunk_vectors {
        string chunk_id PK
        vector embedding
    }
    
    document_versions {
        int version PK
        string document_id FK
        string hash
        int chunk_count
        datetime created_at
    }
    
    jobs {
        string id PK
        string type
        string status
        int progress
        string current_stage
        datetime created_at
        datetime updated_at
    }
```

### Qdrant 存储结构

```mermaid
graph TB
    subgraph "Collection: rag_chunks"
        Point[Point<br/>id: chunk_id<br/>vector: embedding<br/>payload: metadata]
    end
    
    Point -->|metadata| DocID[document_id]
    Point -->|metadata| Content[content]
    Point -->|metadata| SectionPath[section_path]
    Point -->|metadata| Index[index]
```

## 部署架构

### 单机部署

```mermaid
graph TB
    subgraph "单进程"
        Server[Go Server]
        SQLite[(SQLite DB)]
        Memory[内存队列]
    end
    
    Server --> SQLite
    Server --> Memory
    Server --> OpenAI
    Server --> Ollama
```

### 分布式部署

```mermaid
graph TB
    subgraph "负载均衡"
        LB[Load Balancer]
    end
    
    subgraph "应用层"
        Server1[Server 1]
        Server2[Server 2]
        Server3[Server N]
    end
    
    subgraph "存储层"
        Qdrant[Qdrant Cluster]
        Redis[Redis Cluster]
        PostgreSQL[(PostgreSQL<br/>可选)]
    end
    
    subgraph "外部服务"
        OpenAI[OpenAI]
        Ollama[Ollama]
    end
    
    LB --> Server1
    LB --> Server2
    LB --> Server3
    
    Server1 --> Qdrant
    Server2 --> Qdrant
    Server3 --> Qdrant
    
    Server1 --> Redis
    Server2 --> Redis
    Server3 --> Redis
    
    Server1 --> OpenAI
    Server2 --> Ollama
```

## 技术栈

### 后端

- **语言**: Go 1.21+
- **Web框架**: go-chi/chi
- **数据库**: SQLite + sqlite-vec / Qdrant
- **缓存**: Redis (可选)
- **任务队列**: 内存队列 / Redis (asynq)
- **日志**: zap
- **配置**: viper

### 外部服务

- **LLM**: OpenAI, Anthropic, Ollama, Kimi
- **Embedding**: OpenAI, Ollama
- **Reranker**: Cohere

### 监控

- **Metrics**: Prometheus
- **健康检查**: 内置健康检查端点

## 设计模式

### 1. 注册表模式 (Registry Pattern)

用于管理多个 Provider：
- `ParserRegistry`: 文档解析器
- `EmbedderRegistry`: Embedding provider
- `LLMRegistry`: LLM provider

### 2. 策略模式 (Strategy Pattern)

用于可替换的算法：
- 分块策略（结构感知、语义分块）
- 检索策略（向量、BM25、混合）
- 融合策略（RRF、Average、Max）

### 3. 适配器模式 (Adapter Pattern)

用于接口适配：
- `VectorStore` 接口的多种实现（SQLite、Qdrant）
- `Cache` 接口的多种实现（内存、Redis）

### 4. Worker Pool 模式

用于并发处理：
- 任务队列的 Worker Pool
- 并发安全的资源访问

## 扩展点

### 1. 新文档格式

实现 `Parser` 接口并注册到 `ParserRegistry`

### 2. 新存储后端

实现 `VectorStore` 接口

### 3. 新 LLM Provider

实现 `LLM` 接口并注册到 `LLMRegistry`

### 4. 新检索策略

实现 `Retriever` 接口

### 5. 新分块策略

实现 `Chunker` 接口

## 性能特性

- **异步处理**: 文档摄取异步化，不阻塞API
- **批量操作**: Embedding 和存储支持批量处理
- **缓存机制**: Embedding 和查询结果缓存
- **并发安全**: 所有模块支持并发访问
- **连接池**: HTTP 客户端连接复用

## 可靠性特性

- **重试机制**: 网络错误自动重试
- **熔断器**: 防止级联失败
- **限流**: API 限流保护
- **超时控制**: 所有外部调用都有超时
- **优雅关闭**: 支持优雅关闭和资源清理

