# RAG Knowledge Base - Flutter Frontend

Flutter前端应用，用于RAG知识库系统。

## 功能特性

- ✅ 健康检查状态显示（API、Embedding、LLM）
- ✅ 对话管理（创建、选择、删除对话）
- ✅ 多轮对话（发送消息、接收回复）
- ✅ 文档上传（文件选择器）
- ✅ 文档列表显示
- ✅ 引用显示（Citations）
- ✅ Markdown渲染
- ✅ 实时状态更新

## 安装和运行

### 前置要求

- Flutter SDK 3.0.0 或更高版本
- Dart SDK

### 安装依赖

```bash
cd frontend/rag_flutter
flutter pub get
```

### 运行应用

```bash
# 开发模式
flutter run

# 指定设备
flutter run -d chrome  # Web
flutter run -d windows # Windows
flutter run -d linux   # Linux
flutter run -d macos   # macOS
```

### 构建应用

```bash
# Web
flutter build web

# Windows
flutter build windows

# Linux
flutter build linux

# macOS
flutter build macos
```

## 配置

默认API地址为 `http://localhost:8080/api/v1`。

如需修改，编辑 `lib/services/api_service.dart` 中的 `baseUrl` 常量。

## 项目结构

```
lib/
├── main.dart                 # 应用入口
├── models/
│   └── api_models.dart      # API数据模型
├── services/
│   └── api_service.dart     # API服务
├── providers/
│   ├── app_state.dart       # 应用状态（健康检查）
│   ├── conversation_provider.dart  # 对话状态管理
│   └── document_provider.dart      # 文档状态管理
├── screens/
│   └── home_screen.dart     # 主界面
└── widgets/
    ├── conversation_sidebar.dart   # 对话侧边栏
    ├── chat_area.dart              # 聊天区域
    ├── document_sidebar.dart       # 文档侧边栏
    └── upload_modal.dart           # 上传对话框
```

## API端点

应用使用以下后端API端点：

- `GET /api/v1/health` - 健康检查
- `GET /api/v1/conversations` - 获取对话列表
- `POST /api/v1/conversations` - 创建对话
- `GET /api/v1/conversations/{id}` - 获取对话详情
- `DELETE /api/v1/conversations/{id}` - 删除对话
- `POST /api/v1/conversations/{id}/messages` - 发送消息
- `GET /api/v1/documents` - 获取文档列表
- `POST /api/v1/documents` - 上传文档
- `DELETE /api/v1/documents/{id}` - 删除文档
- `POST /api/v1/query` - 单轮查询（未使用，使用对话API）

## 开发说明

### 状态管理

使用 `provider` 进行状态管理：

- `AppState`: 管理应用全局状态（健康检查）
- `ConversationProvider`: 管理对话相关状态
- `DocumentProvider`: 管理文档相关状态

### 主题

应用使用深色主题，颜色方案与原始HTML前端保持一致。

### 依赖包

- `http`: HTTP客户端
- `provider`: 状态管理
- `file_picker`: 文件选择
- `flutter_markdown`: Markdown渲染

## 许可证

与主项目相同。

