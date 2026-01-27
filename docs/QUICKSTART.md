# RAG系统快速启动指南

## 一键启动（推荐）

```bash
# 启动所有服务
./scripts/start-all.sh

# 停止所有服务
./scripts/stop-all.sh
```

## 手动启动步骤

### 1. 启动Ollama（终端1）

```bash
# 启动Ollama服务
ollama serve

# 下载必要的模型（如果未下载）
ollama pull nomic-embed-text  # 嵌入模型
ollama pull llama2            # LLM模型
```

### 2. 启动后端（终端2）

```bash
cd /home/krc/RAG

# 使用Ollama配置启动后端
go run ./cmd/server --config configs/config-ollama.yaml
```

验证后端：
```bash
curl http://localhost:8080/api/v1/health
```

### 3. 启动Flutter前端（终端3）

```bash
cd /home/krc/RAG/frontend/rag_flutter

# 安装依赖（首次运行）
flutter pub get

# 运行应用
flutter run -d chrome
```

## 验证系统运行

1. **检查Ollama**: `curl http://localhost:11434/api/tags`
2. **检查后端**: `curl http://localhost:8080/api/v1/health`
3. **检查前端**: 浏览器自动打开Flutter应用

## 测试流程

1. 在Flutter应用中点击"Upload Document"
2. 上传一个测试文档（.md, .txt, .pdf）
3. 等待文档处理完成
4. 在聊天框中输入问题
5. 查看AI回复和引用

## 常见问题

- **Ollama连接失败**: 确保 `ollama serve` 正在运行
- **后端启动失败**: 检查端口8080是否被占用
- **Flutter运行失败**: 运行 `flutter doctor` 检查环境

详细文档: `docs/运行指南.md`
