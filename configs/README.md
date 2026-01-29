# 配置文件说明

本目录包含多个配置文件，用于不同的部署场景。**目前仅支持 Ollama 和 Kimi API**。

## 配置文件列表

### 1. `config.yaml` - 默认配置
**用途**: 默认配置，使用 Kimi LLM + Ollama Embedding

**特点**:
- LLM: Kimi API (moonshot-v1-8k)
- Embedding: Ollama (nomic-embed-text)
- 数据库: Qdrant

**使用方法**:
```bash
go run ./cmd/server
# 或
make run
```

---

### 2. `config-ollama.yaml` - 纯本地配置
**用途**: 完全本地运行，无需任何 API Key

**特点**:
- LLM: Ollama (llama3.2)
- Embedding: Ollama (nomic-embed-text)
- 数据库: Qdrant
- 适合: 本地开发、测试、无网络环境

**前置要求**:
1. 启动 Ollama: `ollama serve`
2. 下载模型: `ollama pull nomic-embed-text`
3. 下载模型: `ollama pull llama3.2`
4. 启动 Qdrant: `docker-compose up -d qdrant`

**使用方法**:
```bash
make run-config CONFIG=./configs/config-ollama.yaml
# 或
go run ./cmd/server -config ./configs/config-ollama.yaml
```

---

### 3. `config-kimi.yaml` - Kimi API 配置
**用途**: 使用 Kimi API 作为 LLM，Ollama 作为 Embedding

**特点**:
- LLM: Kimi API (moonshot-v1-8k)
- Embedding: Ollama (nomic-embed-text)
- 数据库: Qdrant
- 适合: 需要高质量中文 LLM 的场景

**前置要求**:
1. 启动 Ollama: `ollama serve` (用于 embedding)
2. 下载模型: `ollama pull nomic-embed-text`
3. 启动 Qdrant: `docker-compose up -d qdrant`
4. 配置 Kimi API Key（已在配置文件中）

**使用方法**:
```bash
make run-config CONFIG=./configs/config-kimi.yaml
# 或
go run ./cmd/server -config ./configs/config-kimi.yaml
```

---

### 4. `config-qdrant.yaml` - Qdrant 配置
**用途**: 与 `config-kimi.yaml` 相同，保留用于向后兼容

**特点**:
- LLM: Kimi API
- Embedding: Ollama
- 数据库: Qdrant

**使用方法**:
```bash
make run-config CONFIG=./configs/config-qdrant.yaml
```

---

### 5. `config-mysql-qdrant.yaml` - MySQL + Qdrant 混合存储
**用途**: 生产环境配置，使用 MySQL 存储元数据，Qdrant 存储向量

**特点**:
- LLM: Kimi API
- Embedding: Ollama
- 数据库: MySQL (元数据) + Qdrant (向量)
- 适合: 生产环境、需要事务支持、需要复杂查询

**前置要求**:
1. 启动 Docker 服务: `docker-compose up -d`
2. 启动 Ollama: `ollama serve`
3. 下载模型: `ollama pull nomic-embed-text`

**使用方法**:
```bash
make run-mysql-qdrant
# 或
go run ./cmd/server -config ./configs/config-mysql-qdrant.yaml
```

---

## 配置对比表

| 配置 | LLM | Embedding | 数据库 | 适用场景 |
|------|-----|-----------|--------|----------|
| `config.yaml` | Kimi | Ollama | Qdrant | 默认配置 |
| `config-ollama.yaml` | Ollama | Ollama | Qdrant | 本地开发 |
| `config-kimi.yaml` | Kimi | Ollama | Qdrant | 中文场景 |
| `config-qdrant.yaml` | Kimi | Ollama | Qdrant | 向后兼容 |
| `config-mysql-qdrant.yaml` | Kimi | Ollama | MySQL+Qdrant | 生产环境 |

---

## 快速选择指南

### 场景 1: 本地开发测试
→ 使用 `config-ollama.yaml`
- 无需 API Key
- 完全本地运行
- 适合快速测试

### 场景 2: 需要高质量中文 LLM
→ 使用 `config-kimi.yaml` 或 `config.yaml`
- Kimi API 对中文支持好
- 响应速度快
- 需要 API Key

### 场景 3: 生产环境部署
→ 使用 `config-mysql-qdrant.yaml`
- MySQL 提供事务支持
- Qdrant 提供向量搜索
- 适合生产环境

---

## 注意事项

1. **Ollama 模型**: 使用 Ollama 前需要先下载模型
   ```bash
   ollama pull nomic-embed-text  # Embedding 模型
   ollama pull llama3.2          # LLM 模型
   ```

2. **Kimi API Key**: 配置文件中的 API Key 是示例，生产环境建议使用环境变量
   ```bash
   export KIMI_API_KEY="your-api-key"
   ```

3. **数据库初始化**: 
   - Qdrant 会自动创建 collection
   - MySQL 需要先创建数据库（Docker Compose 会自动创建）

4. **服务依赖**: 
   - Qdrant: `docker-compose up -d qdrant`
   - MySQL: `docker-compose up -d mysql`
   - Ollama: `ollama serve`

---

## 环境变量支持

配置文件支持环境变量替换，格式: `${VAR_NAME}`

例如:
```yaml
kimi:
  api_key: "${KIMI_API_KEY}"  # 从环境变量读取
```

---

## 配置验证

启动前可以验证配置文件格式:
```bash
go run ./cmd/server -config ./configs/config-kimi.yaml --validate
```

