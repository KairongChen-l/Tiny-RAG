import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../providers/conversation_provider.dart';
import '../widgets/conversation_drawer.dart';
import '../widgets/message_list.dart';
import '../widgets/chat_input.dart';

/// 主聊天页面 - ChatGPT 风格的单页面体验
/// 
/// 布局结构：
/// - Scaffold
///   - Drawer: 左侧会话列表（可收起）
///   - Body: 中央对话区域（MessageList）
///   - 底部: 固定输入框（ChatInput）
class ChatPage extends StatefulWidget {
  const ChatPage({super.key});

  @override
  State<ChatPage> createState() => _ChatPageState();
}

class _ChatPageState extends State<ChatPage> {
  final GlobalKey<ScaffoldState> _scaffoldKey = GlobalKey<ScaffoldState>();
  final ScrollController _scrollController = ScrollController();

  @override
  void dispose() {
    _scrollController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    
    return Scaffold(
      key: _scaffoldKey,
      backgroundColor: theme.scaffoldBackgroundColor,
      drawer: const ConversationDrawer(),
      body: SafeArea(
        child: Column(
          children: [
            // 顶部栏（极简，仅菜单按钮）
            _buildTopBar(context, theme),
            
            // 消息列表（占据主要空间）
            Expanded(
              child: MessageList(
                scrollController: _scrollController,
              ),
            ),
            
            // 底部输入区域（固定）
            const ChatInput(),
          ],
        ),
      ),
    );
  }

  Widget _buildTopBar(BuildContext context, ThemeData theme) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
      decoration: BoxDecoration(
        border: Border(
          bottom: BorderSide(
            color: theme.dividerColor,
            width: 1,
          ),
        ),
      ),
      child: Row(
        children: [
          IconButton(
            icon: Icon(
              Icons.menu,
              color: theme.colorScheme.onSurface.withOpacity(0.7),
            ),
            onPressed: () => _scaffoldKey.currentState?.openDrawer(),
            tooltip: '打开会话列表',
          ),
          const SizedBox(width: 8),
          Expanded(
            child: Consumer<ConversationProvider>(
              builder: (context, provider, _) {
                final title = provider.currentConversation?.title ?? '新对话';
                return Text(
                  title,
                  style: TextStyle(
                    fontSize: 16,
                    fontWeight: FontWeight.w500,
                    color: theme.colorScheme.onSurface,
                  ),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                );
              },
            ),
          ),
        ],
      ),
    );
  }
}

