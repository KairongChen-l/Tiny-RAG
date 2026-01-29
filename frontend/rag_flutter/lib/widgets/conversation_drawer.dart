import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../providers/conversation_provider.dart';
import '../providers/app_state.dart';
import '../providers/theme_provider.dart';

/// 左侧会话抽屉 - ChatGPT 风格的会话列表
/// 
/// 功能：
/// - 新建会话
/// - 切换会话
/// - 删除会话
/// - 系统状态显示
class ConversationDrawer extends StatelessWidget {
  const ConversationDrawer({super.key});

  @override
  Widget build(BuildContext context) {
    final conversationProvider = Provider.of<ConversationProvider>(context);
    final appState = Provider.of<AppState>(context);
    final theme = Theme.of(context);

    return Drawer(
      backgroundColor: theme.colorScheme.surface,
      width: 280,
      child: Column(
        children: [
          // 头部：标题和新建按钮
          _buildHeader(context, conversationProvider, theme),
          
          // 会话列表
          Expanded(
            child: _buildConversationsList(context, conversationProvider, theme),
          ),
          
          // 主题切换按钮
          _buildThemeToggle(context, theme),
          
          // 底部状态栏
          _buildStatusBar(context, appState, theme),
        ],
      ),
    );
  }

  Widget _buildHeader(
    BuildContext context,
    ConversationProvider provider,
    ThemeData theme,
  ) {
    return Container(
      padding: const EdgeInsets.all(20),
      decoration: BoxDecoration(
        border: Border(
          bottom: BorderSide(color: theme.dividerColor, width: 1),
        ),
      ),
      child: Column(
        children: [
          Row(
            children: [
              Icon(
                Icons.chat_bubble_outline,
                color: theme.colorScheme.primary,
                size: 24,
              ),
              const SizedBox(width: 12),
              Text(
                'RAG 知识库',
                style: TextStyle(
                  fontSize: 20,
                  fontWeight: FontWeight.w600,
                  color: theme.colorScheme.onSurface,
                ),
              ),
            ],
          ),
          const SizedBox(height: 16),
          SizedBox(
            width: double.infinity,
            child: OutlinedButton.icon(
              onPressed: () {
                provider.createConversation();
                Navigator.pop(context);
              },
              icon: const Icon(Icons.add, size: 18),
              label: const Text('新建对话'),
              style: OutlinedButton.styleFrom(
                foregroundColor: theme.colorScheme.onSurface,
                side: BorderSide(color: theme.dividerColor),
                padding: const EdgeInsets.symmetric(vertical: 12),
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildConversationsList(
    BuildContext context,
    ConversationProvider provider,
    ThemeData theme,
  ) {
    if (provider.isLoading && provider.conversations.isEmpty) {
      return Center(
        child: CircularProgressIndicator(
          color: theme.colorScheme.primary,
        ),
      );
    }

    if (provider.conversations.isEmpty) {
      return Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              Icons.chat_bubble_outline,
              size: 48,
              color: theme.colorScheme.onSurface.withOpacity(0.5),
            ),
            const SizedBox(height: 16),
            Text(
              '还没有对话',
              style: TextStyle(
                color: theme.colorScheme.onSurface.withOpacity(0.7),
                fontSize: 14,
              ),
            ),
            const SizedBox(height: 8),
            TextButton(
              onPressed: () {
                provider.createConversation();
                Navigator.pop(context);
              },
              child: const Text('创建第一个对话'),
            ),
          ],
        ),
      );
    }

    return ListView.builder(
      padding: const EdgeInsets.symmetric(vertical: 8),
      itemCount: provider.conversations.length,
      itemBuilder: (context, index) {
        final conv = provider.conversations[index];
        final isSelected = provider.currentConversation?.id == conv.id;

        return _ConversationItem(
          conversation: conv,
          isSelected: isSelected,
          theme: theme,
          onTap: () {
            provider.selectConversation(conv.id);
            Navigator.pop(context);
          },
          onDelete: () => _handleDelete(context, provider, conv.id),
        );
      },
    );
  }

  Widget _buildThemeToggle(BuildContext context, ThemeData theme) {
    final themeProvider = Provider.of<ThemeProvider>(context);
    
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
      decoration: BoxDecoration(
        border: Border(
          top: BorderSide(color: theme.dividerColor, width: 1),
          bottom: BorderSide(color: theme.dividerColor, width: 1),
        ),
      ),
      child: ListTile(
        dense: true,
        leading: Icon(
          themeProvider.isDarkMode ? Icons.dark_mode : Icons.light_mode,
          size: 20,
          color: theme.colorScheme.onSurface,
        ),
        title: Text(
          themeProvider.isDarkMode ? '深色模式' : '浅色模式',
          style: TextStyle(
            fontSize: 14,
            color: theme.colorScheme.onSurface,
          ),
        ),
        trailing: Switch(
          value: themeProvider.isDarkMode,
          onChanged: (_) => themeProvider.toggleTheme(),
          activeColor: theme.colorScheme.primary,
        ),
        onTap: () => themeProvider.toggleTheme(),
      ),
    );
  }

  Widget _buildStatusBar(BuildContext context, AppState appState, ThemeData theme) {
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        border: Border(
          top: BorderSide(color: theme.dividerColor, width: 1),
        ),
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          _buildStatusItem('API', appState.isApiOnline, theme),
          const SizedBox(height: 8),
          _buildStatusItem('Embedding', appState.isEmbeddingOk, theme),
          const SizedBox(height: 8),
          _buildStatusItem('LLM', appState.isLlmOk, theme),
        ],
      ),
    );
  }

  Widget _buildStatusItem(String label, bool isOk, ThemeData theme) {
    return Row(
      children: [
        Container(
          width: 8,
          height: 8,
          decoration: BoxDecoration(
            color: isOk ? const Color(0xFF3fb950) : const Color(0xFFf85149),
            shape: BoxShape.circle,
          ),
        ),
        const SizedBox(width: 8),
        Text(
          label,
          style: TextStyle(
            fontSize: 12,
            color: theme.colorScheme.onSurface.withOpacity(0.7),
          ),
        ),
      ],
    );
  }

  void _handleDelete(
    BuildContext context,
    ConversationProvider provider,
    String id,
  ) {
    // 通过消息形式反馈，而不是弹窗
    provider.deleteConversation(id);
    if (context.mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text('会话已删除'),
          duration: Duration(seconds: 2),
          backgroundColor: Color(0xFF21262d),
        ),
      );
    }
  }
}

class _ConversationItem extends StatelessWidget {
  final dynamic conversation;
  final bool isSelected;
  final ThemeData theme;
  final VoidCallback onTap;
  final VoidCallback onDelete;

  const _ConversationItem({
    required this.conversation,
    required this.isSelected,
    required this.theme,
    required this.onTap,
    required this.onDelete,
  });

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      child: Container(
        margin: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
        decoration: BoxDecoration(
          color: isSelected ? theme.cardColor : Colors.transparent,
          borderRadius: BorderRadius.circular(8),
        ),
        child: Row(
          children: [
            Icon(
              Icons.chat_bubble_outline,
              size: 16,
              color: theme.colorScheme.onSurface.withOpacity(0.7),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Text(
                conversation.title,
                style: TextStyle(
                  color: isSelected
                      ? theme.colorScheme.onSurface
                      : theme.colorScheme.onSurface.withOpacity(0.7),
                  fontSize: 14,
                ),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
              ),
            ),
            if (isSelected)
              IconButton(
                icon: Icon(Icons.delete_outline, size: 18),
                color: theme.colorScheme.onSurface.withOpacity(0.7),
                onPressed: onDelete,
                tooltip: '删除会话',
              ),
          ],
        ),
      ),
    );
  }
}

