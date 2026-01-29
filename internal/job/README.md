# 异步任务模块 (Job)

## 模块职责

异步任务模块负责管理后台异步任务的处理，包括任务提交、状态追踪、进度更新和错误处理。支持内存队列和 Redis 队列两种实现。

## 核心功能

- **异步处理**：后台处理耗时任务（如文档摄取）
- **状态追踪**：实时追踪任务状态和进度
- **进度更新**：支持阶段化进度更新和历史记录
- **错误处理**：任务失败时的错误记录和重试
- **Worker Pool**：并发处理多个任务

## 核心数据结构

### Job

```go
type Job struct {
    ID              string            // 唯一标识符
    Type            JobType           // 任务类型
    Status          JobStatus         // 任务状态
    Progress        int               // 进度（0-100）
    CurrentStage    string            // 当前阶段标识
    StageMessage    string            // 阶段消息
    ProgressHistory []ProgressEntry   // 进度历史
    Error           string            // 错误信息
    Result          string            // 结果信息
    CreatedAt       time.Time         // 创建时间
    UpdatedAt       time.Time         // 更新时间
    ProgressCallback func(*Job)        // 进度回调
}
```

### JobStatus

```go
type JobStatus string

const (
    StatusPending    JobStatus = "pending"    // 等待处理
    StatusProcessing JobStatus = "processing" // 处理中
    StatusCompleted  JobStatus = "completed"  // 已完成
    StatusFailed     JobStatus = "failed"     // 失败
)
```

### JobType

```go
type JobType string

const (
    TypeDocumentIngest JobType = "document_ingest" // 文档摄取
)
```

### ProgressEntry

```go
type ProgressEntry struct {
    Progress     int       // 进度值
    Stage        string    // 阶段标识
    StageMessage string    // 阶段消息
    Timestamp    time.Time // 时间戳
}
```

## 核心接口

### Queue

```go
type Queue struct {
    jobs    chan *Job
    workers int
    handler Handler
    store   Store
    logger  *zap.Logger
}

// Handler 处理任务
type Handler func(ctx context.Context, job *Job) error

// Store 持久化任务状态
type Store interface {
    Create(ctx context.Context, job *Job) error
    Get(ctx context.Context, id string) (*Job, error)
    Update(ctx context.Context, job *Job) error
}
```

## 使用示例

### 创建队列

```go
// 创建配置
cfg := job.QueueConfig{
    Workers:   10,
    QueueSize: 100,
}

// 创建存储（SQLite）
store := sqlite.NewJobStore(db)

// 创建队列
queue := job.NewQueue(cfg, store, handleJob, logger)

// 启动队列
ctx := context.Background()
queue.Start(ctx)

// 停止队列（优雅关闭）
defer queue.Stop()
```

### 提交任务

```go
// 创建任务
job := &job.Job{
    ID:   uuid.New().String(),
    Type: job.TypeDocumentIngest,
}

// 提交任务
err := queue.Submit(ctx, job)
if err != nil {
    return err
}

// 获取任务ID
jobID := job.ID
```

### 处理任务

```go
func handleJob(ctx context.Context, j *job.Job) error {
    // 设置进度回调
    j.SetProgressWithStage(10, "parsing", "解析文档")
    
    // 处理逻辑
    doc, err := parseDocument(j.Data)
    if err != nil {
        return err
    }
    
    j.SetProgressWithStage(30, "chunking", "分块处理")
    chunks, err := chunkDocument(doc)
    
    j.SetProgressWithStage(50, "embedding", "生成向量")
    vectors, err := embedChunks(chunks)
    
    j.SetProgressWithStage(80, "indexing", "存储索引")
    err = storeChunks(vectors)
    
    j.SetProgressWithStage(100, "complete", "处理完成")
    return nil
}
```

### 查询任务状态

```go
// 从存储获取任务
job, err := store.Get(ctx, jobID)
if err != nil {
    return err
}

fmt.Printf("Status: %s\n", job.Status)
fmt.Printf("Progress: %d%%\n", job.Progress)
fmt.Printf("Stage: %s\n", job.CurrentStage)
fmt.Printf("Message: %s\n", job.StageMessage)

// 查看进度历史
for _, entry := range job.ProgressHistory {
    fmt.Printf("[%s] %d%% - %s: %s\n", 
        entry.Timestamp, entry.Progress, entry.Stage, entry.StageMessage)
}
```

## 实现方式

### 1. 内存队列

**实现**：`internal/job/queue.go`

**特点**：
- 基于 Go channel
- 简单高效
- 进程重启会丢失队列任务

**适用场景**：
- 单进程部署
- 任务可丢失
- 简单场景

### 2. Redis 队列

**实现**：`internal/job/queue_redis.go`

**特点**：
- 使用 asynq 库
- 任务持久化
- 支持分布式部署
- 自动重试机制

**配置**：
```yaml
job:
  use_redis: true
  redis:
    addr: "localhost:6379"
    db: 0
    concurrency: 10
    max_retries: 3
```

**适用场景**：
- 多进程部署
- 任务需要持久化
- 生产环境

## 设计决策

### 1. Worker Pool 模式

- **实现**：固定数量的 worker goroutines
- **优势**：控制并发数，避免资源耗尽
- **配置**：可通过 `QueueConfig.Workers` 调整

### 2. 进度追踪

- **实现**：阶段化进度更新
- **优势**：用户可实时了解任务进度
- **历史**：保留最近 20 条进度记录

### 3. 状态持久化

- **实现**：通过 `Store` 接口持久化
- **优势**：进程重启后状态不丢失
- **实现**：SQLite、Redis 等

### 4. 进度回调

- **实现**：自动持久化进度更新
- **优势**：实时更新，不阻塞处理
- **错误处理**：回调失败不影响任务执行

## 任务处理流程

```
1. 提交任务 → 持久化到 Store
2. 加入队列 → channel 或 Redis
3. Worker 获取任务
4. 更新状态为 processing
5. 执行 Handler
   - 设置进度和阶段
   - 处理业务逻辑
6. 更新状态为 completed/failed
7. 持久化最终状态
```

## 进度阶段示例

文档摄取任务的典型阶段：

- **10%**: `parsing` - 解析文档
- **30%**: `checking` - 检查文档是否已存在
- **40%**: `chunking` - 分块处理
- **50%**: `embedding` - 生成 embeddings
- **80%**: `indexing` - 存储文档和 chunks
- **100%**: `complete` - 处理完成

## 错误处理

### 任务失败

```go
// Handler 返回错误
if err := processJob(ctx, job); err != nil {
    job.SetFailed(err.Error())
    return err
}
```

### 重试机制

- **Redis 队列**：支持自动重试（可配置次数）
- **内存队列**：需要手动实现重试逻辑

## 依赖关系

- **输入**：任务数据（通过 `Job` 结构）
- **依赖**：`Store` 接口（持久化）
- **输出**：任务状态和结果（供 API 查询）

## 测试

运行测试：

```bash
go test ./internal/job/... -v
```

测试覆盖：
- 任务提交和获取
- 状态更新
- 进度追踪
- 错误处理
- Worker Pool
- 上下文取消

## 监控指标

- `job_submissions_total`：任务提交总数
- `job_completions_total`：任务完成总数
- `job_failures_total`：任务失败总数
- `job_duration_seconds`：任务处理耗时
- `job_queue_size`：队列大小

