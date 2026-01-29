# Changelog - v2.0.0

## 版本概述

v2.0.0 是一个重要的工程级重构版本，完成了三个阶段的系统性改进，显著提升了代码质量、架构清晰度和功能完整性。

**发布日期**: 2026-01-29

## 主要变更

### Phase 1: 基础设施重构

#### Bootstrap 初始化逻辑重构
- **新增**: `internal/bootstrap/config.go` - 配置验证逻辑
- **新增**: `internal/bootstrap/lifecycle.go` - 分阶段生命周期管理
  - `initInfrastructure()` - 基础设施初始化
  - `initBusinessComponents()` - 业务组件初始化
  - `initServiceLayer()` - 服务层初始化
  - `initAPILayer()` - API 层初始化
- **重构**: `internal/bootstrap/app.go` - 从 200+ 行简化到 50 行以内

#### Kafka Consumer 实现
- **新增**: `internal/mq/kafka/consumer.go` - 基于 segmentio/kafka-go 的 Consumer
  - 支持手动提交 offset
  - 实现优雅关停
- **完善**: `internal/mq/consumer_registry.go` - 多消费者管理
- **新增**: `internal/mq/consumer_handlers.go` - 文档处理 Consumer Handler

#### 优雅关停完善
- WebSocket Hub 实现 `Close()` 方法
- Kafka Consumer 实现 `Resource` 接口
- 所有资源注册到 ResourceManager

### Phase 2: 架构优化

#### Repository 层引入
- **新增**: `internal/repository/document_repository.go`
- **新增**: `internal/repository/job_repository.go`
- **新增**: `internal/repository/conversation_repository.go`
- **重构**: Service 层使用 Repository，提升代码可测试性

#### Handler 重构
- Handler 结构体添加 Service 层依赖
- Query 方法改为调用 SearchService
- Conversation 相关方法改为调用 ConversationService
- 保持向后兼容，支持 legacy 实现作为 fallback

#### ConsumerRegistry 实现
- 在 Bootstrap 中初始化 ConsumerRegistry
- 注册文档处理 Consumer Handler（ingested, uploaded, failed）
- 在 app.go 中启动 ConsumerRegistry

### Phase 3: 功能完善

#### LLM 流式输出
- **新增**: OpenAI `GenerateStream` 方法
- **新增**: Ollama `GenerateStream` 方法
- **新增**: Anthropic `GenerateStream` 方法
- **新增**: Kimi `GenerateStream` 方法
- Service 层支持流式输出（SearchService.SearchStream, ConversationService.SendMessageStream）

#### ES Mapping 多租户支持
- Document 结构添加 `userId`、`orgTag`、`isPublic` 字段
- 更新 ES mapping 添加多租户字段
- Search 方法支持 SearchOptions 进行多租户过滤
- 默认逻辑：显示公开文档或用户自己的文档

#### Apache Tika 集成
- **新增**: `internal/ingestion/tika.go` - Tika HTTP 服务客户端
- 支持格式：Word (DOCX/DOC), Excel (XLSX/XLS), PowerPoint (PPTX/PPT), HTML, XML, RTF, ODT
- 添加新格式常量
- 在 Bootstrap 中注册 Tika parser（如果启用）

#### Docker Compose 完善
- **新增**: `docker-compose.full.yml` - 完整环境配置
- **更新**: `docker-compose.integrations.yml` - 添加 Tika 和 Elasticsearch
- 所有服务配置健康检查

## 架构改进

### 代码结构
- Bootstrap 从 200+ 行简化到 50 行以内
- 模块边界清晰，职责明确
- 分层架构完善（Repository -> Service -> Handler）

### 依赖管理
- Handler 不再直接依赖 VectorStore、config 等基础设施
- 数据访问层与业务逻辑层分离
- 便于后续替换存储实现

### 可维护性
- 代码可测试性提升
- 模块化程度提高
- 文档完善（各模块 README）

## 功能增强

### 文档格式支持
- 新增支持：Word, Excel, PowerPoint, HTML, XML, RTF, ODT
- 统一使用 Tika 处理复杂格式

### 流式输出
- 所有 LLM Provider 统一支持流式输出
- 支持实时流式响应，提升用户体验

### 多租户
- ES 支持多租户查询
- 数据隔离能力提升

### 消息队列
- Kafka 消息处理自动化
- 支持文档处理事件流

## 依赖更新

- 添加 `github.com/segmentio/kafka-go v0.4.50`

## 配置变更

### 新增配置项
- `ingestion.tika.enabled` - 启用 Tika 解析器
- `ingestion.tika.base_url` - Tika 服务地址
- `ingestion.tika.timeout` - Tika 请求超时
- `llm.*.timeout` - 各 LLM Provider 超时配置

## 破坏性变更

无。所有变更保持向后兼容。

## 迁移指南

### 启用 Tika
在配置文件中添加：
```yaml
ingestion:
  tika:
    enabled: true
    base_url: "http://localhost:9998"
    timeout: 60s
```

### 启用 Kafka Consumer
在配置文件中启用 Kafka，ConsumerRegistry 会自动启动：
```yaml
messaging:
  enabled: true
  kafka:
    enabled: true
    brokers: ["localhost:9092"]
```

### 使用多租户 ES 查询
```go
opts := search.SearchOptions{
    UserID: "user123",
    OrgTag: "org1",
    IsPublic: &isPublic,
}
results, err := esClient.Search(ctx, query, size, opts)
```

## 提交统计

- **总提交数**: 17 个功能提交
- **新增文件**: 50+ 个
- **修改文件**: 30+ 个
- **删除文件**: 10+ 个（清理未使用代码）

## 主要提交

1. `refactor(bootstrap)`: Phase 1 - 重构 Bootstrap 初始化逻辑和 Kafka Consumer
2. `feat(repository)`: Phase 2 - 引入 Repository 层
3. `refactor(handler)`: Phase 2 - 重构 Handler 移除直接依赖
4. `feat(generation)`: Phase 3 - 实现 LLM 流式输出
5. `feat(search)`: Phase 3 - 完善 ES Mapping 支持多租户
6. `feat(ingestion)`: Phase 3 - 集成 Apache Tika 文档解析
7. `feat(docker)`: Phase 3 - 完善 Docker Compose 配置

## 测试

- 所有现有测试通过
- 新增测试覆盖新功能
- 编译无错误

## 下一步

- 继续完善测试覆盖
- 性能优化
- 监控和可观测性增强

