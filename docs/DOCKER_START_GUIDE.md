# Docker 容器启动指南

本指南说明如何启动 RAG 系统所需的所有 Docker 容器。

## 快速开始

### 方式一：启动所有服务（推荐）

使用 `docker-compose.full.yml` 一键启动所有服务：

```bash
docker-compose -f docker-compose.full.yml up -d
```

这将启动以下服务：
- MySQL (端口 3306)
- Redis (端口 6379)
- Qdrant (端口 6333, 6334)
- Zookeeper (端口 2181)
- Kafka (端口 9092, 9093)
- MinIO (端口 9000, 9001)
- Apache Tika (端口 9998)
- Elasticsearch (端口 9200, 9300)

### 方式二：分步启动

#### 1. 启动核心服务

```bash
docker-compose -f docker-compose.yml up -d
```

核心服务包括：
- MySQL - 存储文档元数据、chunks、jobs
- Redis - 缓存和任务队列
- Qdrant - 向量存储和相似度搜索

#### 2. 启动可选集成服务

```bash
docker-compose -f docker-compose.yml -f docker-compose.integrations.yml up -d
```

可选服务包括：
- Kafka + Zookeeper - 消息队列
- MinIO - 对象存储
- Apache Tika - 文档解析服务
- Elasticsearch - 全文搜索服务

## 使用启动脚本

### 启动所有服务

```bash
./scripts/start-all.sh
```

### 仅启动核心服务

```bash
./scripts/start-services.sh
```

## 服务端口说明

| 服务 | 端口 | 说明 |
|------|------|------|
| MySQL | 3306 | 数据库服务 |
| Redis | 6379 | 缓存服务 |
| Qdrant | 6333, 6334 | 向量数据库（HTTP, gRPC） |
| Zookeeper | 2181 | Kafka 依赖 |
| Kafka | 9092, 9093 | 消息队列（外部, 内部） |
| MinIO | 9000, 9001 | 对象存储（API, Console） |
| Tika | 9998 | 文档解析服务 |
| Elasticsearch | 9200, 9300 | 全文搜索（HTTP, Transport） |

## 服务访问地址

### Web 界面

- **Qdrant Dashboard**: http://localhost:6333/dashboard
- **MinIO Console**: http://localhost:9001
  - 用户名: `minioadmin`
  - 密码: `minioadmin`
- **Elasticsearch**: http://localhost:9200

### API 端点

- **Tika**: http://localhost:9998/tika
- **Kafka**: localhost:9092

## 检查服务状态

### 查看所有容器状态

```bash
docker-compose -f docker-compose.full.yml ps
```

### 查看服务日志

```bash
# 查看所有服务日志
docker-compose -f docker-compose.full.yml logs -f

# 查看特定服务日志
docker-compose -f docker-compose.full.yml logs -f mysql
docker-compose -f docker-compose.full.yml logs -f kafka
```

### 检查服务健康状态

```bash
# MySQL
docker exec rag-mysql mysqladmin ping -h localhost

# Redis
docker exec rag-redis redis-cli ping

# Qdrant
curl http://localhost:6333/health

# Kafka
docker exec rag-kafka kafka-broker-api-versions --bootstrap-server localhost:9092

# MinIO
curl http://localhost:9000/minio/health/live

# Tika
curl http://localhost:9998/tika

# Elasticsearch
curl http://localhost:9200/_cluster/health
```

## 停止服务

### 停止所有服务

```bash
docker-compose -f docker-compose.full.yml down
```

### 停止并删除数据卷

```bash
docker-compose -f docker-compose.full.yml down -v
```

⚠️ **警告**: 这将删除所有数据，包括数据库、缓存和存储的数据。

## 重启服务

```bash
docker-compose -f docker-compose.full.yml restart
```

## 常见问题

### 1. 端口冲突

如果遇到端口冲突，可以：

1. 修改 `docker-compose.full.yml` 中的端口映射
2. 停止占用端口的其他服务

### 2. Elasticsearch 内存不足

如果 Elasticsearch 启动失败，可能需要增加 Docker 内存限制：

```yaml
environment:
  - "ES_JAVA_OPTS=-Xms1g -Xmx1g"  # 增加到 1GB
```

### 3. Kafka 启动慢

Kafka 首次启动可能需要较长时间，请耐心等待。可以通过日志查看启动进度：

```bash
docker-compose -f docker-compose.full.yml logs -f kafka
```

### 4. 服务无法连接

确保所有服务在同一个网络 `rag-network` 中：

```bash
docker network ls | grep rag-network
```

## 配置说明

### 环境变量

主要配置在 `docker-compose.full.yml` 中：

- **MySQL**: `MYSQL_ROOT_PASSWORD`, `MYSQL_DATABASE`, `MYSQL_USER`, `MYSQL_PASSWORD`
- **MinIO**: `MINIO_ROOT_USER`, `MINIO_ROOT_PASSWORD`
- **Kafka**: `KAFKA_BROKER_ID`, `KAFKA_ZOOKEEPER_CONNECT`

### 数据持久化

所有数据存储在 Docker volumes 中：

- `mysql_data` - MySQL 数据
- `redis_data` - Redis 数据
- `qdrant_data` - Qdrant 数据
- `kafka_data` - Kafka 数据
- `minio_data` - MinIO 数据
- `es_data` - Elasticsearch 数据

查看 volumes：

```bash
docker volume ls | grep rag
```

## 生产环境建议

1. **修改默认密码**: 更新 MySQL、MinIO 等服务的默认密码
2. **启用安全认证**: 为 Elasticsearch、Kafka 启用安全认证
3. **资源限制**: 为容器设置 CPU 和内存限制
4. **备份策略**: 定期备份 MySQL 和 Elasticsearch 数据
5. **监控**: 配置服务监控和告警

## 下一步

启动服务后，请：

1. 检查配置文件 `configs/config.yaml` 中的服务地址
2. 运行应用程序
3. 验证服务连接

```bash
# 检查配置
cat configs/config.yaml

# 运行应用
go run cmd/server/main.go
```

