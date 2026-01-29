# RAG系统部署文档

## 系统要求

- Go 1.21+
- SQLite 3.x（用于SQLite存储）
- 可选：Qdrant（用于向量存储）
- 可选：Redis（用于缓存和任务队列）

## 快速开始

### 1. 克隆仓库

```bash
git clone <repository-url>
cd RAG
```

### 2. 安装依赖

```bash
go mod download
```

### 3. 配置

复制配置文件并修改：

```bash
cp configs/config.yaml configs/config-local.yaml
```

编辑 `configs/config-local.yaml`，设置：
- Embedding API密钥（OpenAI/Ollama）
- LLM API密钥（OpenAI/Anthropic/Kimi/Ollama）
- 数据库路径

### 4. 运行

```bash
# 使用默认配置
make run

# 使用自定义配置
make run-config CONFIG=./configs/config-local.yaml

# 使用Ollama（本地，无需API密钥）
make run-ollama
```

## 生产部署

### Docker部署

#### 1. 构建镜像

```bash
docker build -t rag-server:latest .
```

#### 2. 运行容器

```bash
docker run -d \
  -p 8080:8080 \
  -v $(pwd)/data:/app/data \
  -v $(pwd)/configs:/app/configs \
  -e OPENAI_API_KEY=your-key \
  rag-server:latest
```

### 系统服务（systemd）

创建 `/etc/systemd/system/rag-server.service`:

```ini
[Unit]
Description=RAG Server
After=network.target

[Service]
Type=simple
User=rag
WorkingDirectory=/opt/rag
ExecStart=/opt/rag/bin/rag-server -config /opt/rag/configs/config.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

启动服务：

```bash
sudo systemctl enable rag-server
sudo systemctl start rag-server
```

## 配置说明

### 数据库配置

#### SQLite（默认）

```yaml
database:
  provider: "sqlite"
  path: "./data/rag.db"
```

#### Qdrant

```yaml
database:
  provider: "qdrant"
  qdrant:
    url: "http://localhost:6333"
    collection: "rag_chunks"
    api_key: ""  # 可选
```

### Embedding配置

#### OpenAI

```yaml
embedding:
  provider: "openai"
  openai:
    api_key: "${OPENAI_API_KEY}"
    model: "text-embedding-3-small"
    dimensions: 1536
```

#### Ollama（本地）

```yaml
embedding:
  provider: "ollama"
  ollama:
    base_url: "http://localhost:11434"
    model: "nomic-embed-text"
    dimensions: 768
```

### LLM配置

#### Kimi（推荐）

```yaml
llm:
  default_provider: "kimi"
  kimi:
    api_key: "your-api-key"
    model: "moonshot-v1-8k"
    max_tokens: 4096
    temperature: 0.7
    base_url: "https://api.moonshot.cn/v1"
```

### 性能监控

```yaml
server:
  performance:
    slow_query_threshold: "1s"  # 慢查询阈值
    enable_tracing: false       # 性能追踪
    log_slow_queries: true      # 记录慢查询
```

### 速率限制

```yaml
server:
  rate_limit:
    enabled: true
    rate: 100       # 每秒请求数
    burst: 100      # 突发请求数
    window: "1s"    # 时间窗口
```

## 监控

### Prometheus Metrics

Metrics端点: `http://localhost:8080/metrics`

主要指标：
- `rag_retrieval_requests_total`: 检索请求总数
- `rag_retrieval_duration_seconds`: 检索耗时
- `rag_llm_requests_total`: LLM请求总数
- `rag_job_completed_total`: 完成任务数

### 慢查询日志

如果启用了慢查询日志，超过阈值的查询会记录到日志中。

## 备份

### 数据库备份

```bash
# SQLite备份
sqlite3 data/rag.db ".backup data/rag.db.backup"

# 定期备份脚本
#!/bin/bash
BACKUP_DIR="/backup/rag"
DATE=$(date +%Y%m%d_%H%M%S)
sqlite3 /opt/rag/data/rag.db ".backup $BACKUP_DIR/rag_$DATE.db"
```

## 故障排查

### 常见问题

1. **端口被占用**
   ```bash
   lsof -i :8080
   ```

2. **数据库锁定**
   - 检查是否有其他进程在使用数据库
   - 检查文件权限

3. **API密钥错误**
   - 检查环境变量
   - 检查配置文件

### 日志

日志输出到标准输出，生产环境建议重定向到文件：

```bash
./rag-server -config config.yaml > /var/log/rag/server.log 2>&1
```

## 性能优化

1. **使用Qdrant替代SQLite**（大规模数据）
2. **启用Redis缓存**（减少API调用）
3. **调整worker数量**（根据CPU核心数）
4. **启用查询缓存**

## 安全建议

1. 使用HTTPS（通过反向代理）
2. 添加API密钥认证
3. 限制文件上传大小
4. 启用速率限制
5. 定期更新依赖

