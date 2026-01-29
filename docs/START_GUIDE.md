# RAG 系统启动指南

## 前置条件

1. **Docker 和 Docker Compose** - 用于启动基础设施服务
2. **Go 1.21+** - 用于编译和运行后端服务
3. **配置文件** - 在 `configs/` 目录下选择合适的配置文件

## 快速启动

### 步骤 1: 启动 Docker 服务

启动所有必需的 Docker 容器：

```bash
# 方式一：使用启动脚本（推荐）
./scripts/start-all.sh

# 方式二：使用 Docker Compose
docker-compose -f docker-compose.full.yml up -d
```

这将启动以下服务：
- MySQL (3306)
- Redis (6379)
- Qdrant (6333, 6334)
- Kafka + Zookeeper (9092, 2181)
- MinIO (9000, 9001)
- Apache Tika (9998)
- Elasticsearch (9200, 9300)

### 步骤 2: 检查服务状态

等待服务启动完成（约 30 秒），然后检查服务状态：

```bash
# 检查所有容器状态
docker-compose -f docker-compose.full.yml ps

# 检查服务健康状态
curl http://localhost:6333/health  # Qdrant
curl http://localhost:9200/_cluster/health  # Elasticsearch
curl http://localhost:9998/tika  # Tika
```

### 步骤 3: 启动后端服务

#### 方式一：使用 Makefile（推荐）

```bash
# 使用默认配置
make run

# 使用指定配置文件
make run-config CONFIG=./configs/config-kimi.yaml
make run-config CONFIG=./configs/config-ollama.yaml
make run-config CONFIG=./configs/config-qdrant.yaml
```

#### 方式二：直接使用 Go 命令

```bash
# 使用默认配置
go run ./cmd/server

# 使用指定配置文件
go run ./cmd/server -config ./configs/config-kimi.yaml
```

#### 方式三：编译后运行

```bash
# 编译
go build -o bin/rag-server ./cmd/server

# 运行
./bin/rag-server -config ./configs/config-kimi.yaml
```

## 配置文件说明

配置文件位于 `configs/` 目录：

- `config.yaml` - 默认配置（SQLite + Qdrant）
- `config-kimi.yaml` - 使用 Kimi (Moonshot) LLM
- `config-ollama.yaml` - 使用 Ollama LLM
- `config-qdrant.yaml` - 使用 Qdrant 向量存储
- `config-mysql-qdrant.yaml` - 使用 MySQL + Qdrant

## 验证启动成功

启动成功后，你应该看到类似以下的日志：

```
{"level":"info","msg":"configuration loaded","port":8080}
{"level":"info","msg":"initialized Qdrant vector store"}
{"level":"info","msg":"initialized Kimi LLM","model":"moonshot-v1-8k"}
{"level":"info","msg":"job queue started","workers":2}
{"level":"info","msg":"initialized Prometheus metrics"}
{"level":"info","msg":"server started","port":8080}
```

然后访问：

- **API 文档**: http://localhost:8080/swagger/index.html
- **健康检查**: http://localhost:8080/health
- **Metrics**: http://localhost:8080/metrics

## 常见问题

### 1. 端口冲突

如果遇到端口冲突，可以：

1. 修改配置文件中的端口
2. 停止占用端口的其他服务

```bash
# 检查端口占用
lsof -i :8080
lsof -i :3306
```

### 2. Docker 服务未启动

确保所有 Docker 服务已启动：

```bash
# 检查服务状态
docker-compose -f docker-compose.full.yml ps

# 查看服务日志
docker-compose -f docker-compose.full.yml logs -f mysql
docker-compose -f docker-compose.full.yml logs -f kafka
```

### 3. 连接数据库失败

检查 MySQL 是否正常运行：

```bash
# 检查 MySQL 连接
docker exec rag-mysql mysqladmin ping -h localhost

# 查看 MySQL 日志
docker-compose -f docker-compose.full.yml logs mysql
```

### 4. Metrics 重复注册错误

如果遇到 "duplicate metrics collector registration attempted" 错误：

✅ **已修复** - 此问题已在 v2.0.0 中修复。如果仍遇到此问题，请确保使用最新代码。

### 5. Kafka 连接失败

确保 Kafka 和 Zookeeper 都已启动：

```bash
# 检查 Kafka 状态
docker exec rag-kafka kafka-broker-api-versions --bootstrap-server localhost:9092

# 查看 Kafka 日志
docker-compose -f docker-compose.full.yml logs kafka
```

## 停止服务

### 停止后端服务

按 `Ctrl+C` 停止运行中的后端服务。

### 停止 Docker 服务

```bash
# 停止所有服务
docker-compose -f docker-compose.full.yml down

# 停止并删除数据卷（⚠️ 会删除所有数据）
docker-compose -f docker-compose.full.yml down -v
```

## 开发模式

### 热重载（使用 air）

```bash
# 安装 air
go install github.com/cosmtrek/air@latest

# 运行（会自动监听文件变化并重启）
air
```

### 调试模式

```bash
# 使用 delve 调试
dlv debug ./cmd/server -- -config ./configs/config-kimi.yaml
```

## 生产环境部署

1. **修改默认密码** - 更新配置文件中的数据库、Redis、MinIO 等密码
2. **启用 HTTPS** - 配置 TLS 证书
3. **设置资源限制** - 为 Docker 容器设置 CPU 和内存限制
4. **配置监控** - 设置 Prometheus 和 Grafana
5. **日志管理** - 配置日志收集和分析

## 下一步

启动成功后，你可以：

1. 查看 API 文档：http://localhost:8080/swagger/index.html
2. 上传文档进行测试
3. 执行查询测试
4. 查看监控指标：http://localhost:8080/metrics

## 获取帮助

- 查看详细文档：`docs/`
- 查看 Docker 启动指南：`docs/DOCKER_START_GUIDE.md`
- 查看架构文档：`docs/architecture.md`

