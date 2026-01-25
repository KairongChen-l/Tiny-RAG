# RAG 系统介绍

## 什么是 RAG？

RAG (Retrieval-Augmented Generation) 是一种结合了信息检索和文本生成的 AI 技术。它通过从知识库中检索相关文档，然后将这些文档作为上下文提供给大语言模型，从而生成更准确、更可靠的回答。

### RAG 的核心优势

1. **减少幻觉**：通过提供真实的文档作为参考，减少 AI 模型凭空生成错误信息的可能性
2. **知识可更新**：无需重新训练模型，只需更新知识库即可更新 AI 的知识
3. **可追溯性**：每个回答都可以追溯到原始文档来源

## 系统架构

### 文档处理流程

文档处理分为以下几个步骤：

1. **文档摄入**：支持 Markdown、PDF、纯文本等格式
2. **结构解析**：保留文档的标题层级结构
3. **智能分块**：按语义边界切分，避免句子被截断
4. **向量化**：使用 Embedding 模型将文本转换为向量
5. **索引存储**：将向量存入数据库以支持相似度搜索

### 查询流程

当用户提出问题时：

1. 将问题转换为向量
2. 在向量数据库中搜索最相关的文档块
3. 将检索到的文档块作为上下文
4. 调用 LLM 生成回答

## 使用指南

### 环境准备

确保已安装以下依赖：

- Go 1.21+
- SQLite3
- Ollama（可选，用于本地测试）

### 启动服务

```bash
# 使用默认配置
make run

# 使用 Ollama 配置
make run-config CONFIG=./configs/config-ollama.yaml
```

### API 使用

#### 上传文档

```bash
curl -X POST http://localhost:8080/api/v1/documents \
  -F "file=@your-document.md"
```

#### 查询问答

```bash
curl -X POST http://localhost:8080/api/v1/query \
  -H "Content-Type: application/json" \
  -d '{"query": "RAG 是什么？"}'
```

## 常见问题

### Q: 为什么检索结果不准确？

可能的原因：
- 分块大小设置不当
- Embedding 模型质量不够
- 相似度阈值设置过高

### Q: 如何提高回答质量？

建议：
- 使用更好的 Embedding 模型
- 调整分块策略
- 增加检索的文档数量
- 使用更强的 LLM 模型

