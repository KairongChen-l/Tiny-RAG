# 版本管理说明

## v2.0.0 版本提交组织

本次工程级重构的提交按照功能模块进行了清晰的划分，便于代码审查和问题追踪。

### 提交策略

所有提交遵循 [Conventional Commits](https://www.conventionalcommits.org/) 规范：

- `refactor`: 代码重构
- `feat`: 新功能
- `chore`: 构建过程或辅助工具的变动
- `docs`: 文档变更
- `fix`: 修复问题

### 提交列表

#### Phase 1: 基础设施重构

1. **refactor(bootstrap)**: Phase 1 - 重构 Bootstrap 初始化逻辑和 Kafka Consumer
   - 创建 config.go 和 lifecycle.go
   - 实现 Kafka Consumer
   - 完善优雅关停

#### Phase 2: 架构优化

2. **feat(repository)**: Phase 2 - 引入 Repository 层
3. **refactor(handler)**: Phase 2 - 重构 Handler 移除直接依赖
4. **feat(bootstrap)**: Phase 2 - 完善 ConsumerRegistry 集成和优雅关停

#### Phase 3: 功能完善

5. **feat(generation)**: Phase 3 - 实现 LLM 流式输出
6. **feat(search)**: Phase 3 - 完善 ES Mapping 支持多租户
7. **feat(ingestion)**: Phase 3 - 集成 Apache Tika 文档解析
8. **feat(docker)**: Phase 3 - 完善 Docker Compose 配置

#### 配置和依赖

9. **chore(deps)**: 更新依赖
10. **feat(config)**: 添加 Tika 和 LLM 超时配置

#### 代码优化

11. **refactor**: 其他代码优化和修复
12. **refactor(index,job)**: 优化 VectorStore 接口和 Job Queue

#### 文档和工具

13. **docs**: 更新实施日志和架构文档
14. **chore**: 更新 Makefile 和静态文件
15. **chore**: 清理未使用的代码和脚本
16. **chore(frontend,docs)**: 前端更新和文档清理
17. **chore**: 添加模块 README 和工具脚本
18. **docs**: 添加 v2.0.0 版本变更日志

### 版本标签

- **v2.0.0**: 工程级重构完成版本
  - 包含所有 Phase 1-3 的功能
  - 架构改进和功能增强
  - 完整的文档和测试

### 统计信息

- **总提交数**: 18 个功能提交
- **文件变更**: 131 个文件
- **代码变更**: +16414 行 / -4318 行
- **新增文件**: 50+ 个
- **删除文件**: 10+ 个（清理未使用代码）

### 查看版本信息

```bash
# 查看版本标签
git tag -l

# 查看 v2.0.0 标签详情
git show v2.0.0

# 查看 v2.0.0 的所有提交
git log v2.0.0^..v2.0.0 --oneline

# 查看版本间的代码变更统计
git diff --stat v0.2.0 v2.0.0
```

### 回滚到特定版本

```bash
# 查看特定版本的代码
git checkout v2.0.0

# 创建基于 v2.0.0 的新分支
git checkout -b hotfix/v2.0.1 v2.0.0
```

### 发布检查清单

- [x] 所有测试通过
- [x] 编译无错误
- [x] 文档更新完整
- [x] 提交信息清晰
- [x] 版本标签创建
- [x] 变更日志编写

### 下一步

如需推送到远程仓库：

```bash
# 推送所有提交
git push origin master

# 推送版本标签
git push origin v2.0.0
```

