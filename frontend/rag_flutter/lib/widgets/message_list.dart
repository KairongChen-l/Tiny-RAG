import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:flutter_markdown/flutter_markdown.dart';
import '../providers/conversation_provider.dart';
import '../models/api_models.dart';

/// 消息列表组件 - 展示对话流
/// 
/// 特点：
/// - 垂直时间流布局
/// - 自动滚动到底部
/// - 支持 Markdown 渲染
/// - 代码块等宽字体
class MessageList extends StatefulWidget {
  final ScrollController scrollController;

  const MessageList({
    super.key,
    required this.scrollController,
  });

  @override
  State<MessageList> createState() => _MessageListState();
}

class _MessageListState extends State<MessageList> {
  @override
  void initState() {
    super.initState();
    // 监听消息变化，自动滚动
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _scrollToBottom();
    });
  }

  void _scrollToBottom() {
    if (widget.scrollController.hasClients) {
      Future.delayed(const Duration(milliseconds: 100), () {
        if (widget.scrollController.hasClients) {
          widget.scrollController.animateTo(
            widget.scrollController.position.maxScrollExtent,
            duration: const Duration(milliseconds: 300),
            curve: Curves.easeOut,
          );
        }
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return Consumer<ConversationProvider>(
      builder: (context, provider, _) {
        final messages = provider.currentConversation?.messages ?? [];
        final isLoading = provider.isLoading;

        // 空状态
        if (messages.isEmpty && !isLoading) {
          return _buildEmptyState();
        }

        // 消息列表
        return ListView.builder(
          controller: widget.scrollController,
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 24),
          itemCount: messages.length + (isLoading ? 1 : 0),
          itemBuilder: (context, index) {
            if (index == messages.length) {
              return const _TypingIndicator();
            }

            final message = messages[index];
            return MessageBubble(message: message);
          },
        );
      },
    );
  }

  Widget _buildEmptyState() {
    final theme = Theme.of(context);
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(
            Icons.chat_bubble_outline,
            size: 64,
            color: theme.colorScheme.onSurface.withOpacity(0.5),
          ),
          const SizedBox(height: 24),
          Text(
            '开始对话',
            style: TextStyle(
              fontSize: 24,
              fontWeight: FontWeight.w600,
              color: theme.colorScheme.onSurface,
            ),
          ),
          const SizedBox(height: 12),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 48),
            child: Text(
              '你可以询问关于文档的问题，或者上传新文档。\n所有操作都可以通过对话完成。',
              textAlign: TextAlign.center,
              style: TextStyle(
                fontSize: 15,
                color: theme.colorScheme.onSurface.withOpacity(0.7),
                height: 1.6,
              ),
            ),
          ),
        ],
      ),
    );
  }
}

/// 消息气泡组件
class MessageBubble extends StatelessWidget {
  final Message message;

  const MessageBubble({super.key, required this.message});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isUser = message.role == 'user';
    final isError = message.status == MessageStatus.error;

    return Padding(
      padding: const EdgeInsets.only(bottom: 32),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisAlignment: isUser ? MainAxisAlignment.end : MainAxisAlignment.start,
        children: [
          if (!isUser) ...[
            _buildAvatar(isUser: false, theme: theme),
            const SizedBox(width: 12),
          ],
          Flexible(
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 800),
              child: Container(
                padding: const EdgeInsets.all(20),
                decoration: BoxDecoration(
                  color: isError
                      ? theme.colorScheme.error.withOpacity(0.1)
                      : isUser
                          ? theme.cardColor
                          : theme.colorScheme.surface,
                  borderRadius: BorderRadius.circular(12),
                  border: Border.all(
                    color: isError
                        ? theme.colorScheme.error
                        : theme.dividerColor,
                    width: 1,
                  ),
                ),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    if (message.status == MessageStatus.loading)
                      const _LoadingIndicator()
                    else
                      _buildContent(theme),
                    if (message.citations != null &&
                        message.citations!.isNotEmpty)
                      _buildCitations(message.citations!, theme),
                  ],
                ),
              ),
            ),
          ),
          if (isUser) ...[
            const SizedBox(width: 12),
            _buildAvatar(isUser: true, theme: theme),
          ],
        ],
      ),
    );
  }

  Widget _buildContent(ThemeData theme) {
    final isDark = theme.brightness == Brightness.dark;
    return MarkdownBody(
      data: message.content,
      styleSheet: MarkdownStyleSheet(
        p: TextStyle(
          color: theme.colorScheme.onSurface,
          fontSize: 15,
          height: 1.7,
        ),
        h1: TextStyle(
          color: theme.colorScheme.onSurface,
          fontSize: 24,
          fontWeight: FontWeight.w600,
          height: 1.5,
        ),
        h2: TextStyle(
          color: theme.colorScheme.onSurface,
          fontSize: 20,
          fontWeight: FontWeight.w600,
          height: 1.5,
        ),
        h3: TextStyle(
          color: theme.colorScheme.onSurface,
          fontSize: 18,
          fontWeight: FontWeight.w600,
          height: 1.5,
        ),
        code: TextStyle(
          backgroundColor: isDark ? const Color(0xFF1f2428) : const Color(0xFFF6F8FA),
          color: theme.colorScheme.onSurface,
          fontFamily: 'monospace',
        ),
        codeblockDecoration: BoxDecoration(
          color: isDark ? const Color(0xFF1f2428) : const Color(0xFFF6F8FA),
          borderRadius: BorderRadius.circular(6),
        ),
        blockquote: TextStyle(
          color: theme.colorScheme.onSurface.withOpacity(0.7),
          fontStyle: FontStyle.italic,
        ),
        listBullet: TextStyle(
          color: theme.colorScheme.onSurface.withOpacity(0.7),
        ),
      ),
    );
  }

  Widget _buildCitations(List<Citation> citations, ThemeData theme) {
    return Padding(
      padding: const EdgeInsets.only(top: 16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            '来源',
            style: TextStyle(
              fontSize: 12,
              fontWeight: FontWeight.w600,
              color: theme.colorScheme.onSurface.withOpacity(0.7),
              letterSpacing: 0.5,
            ),
          ),
          const SizedBox(height: 8),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: citations.map((citation) {
              return Container(
                padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
                decoration: BoxDecoration(
                  color: theme.cardColor,
                  borderRadius: BorderRadius.circular(6),
                  border: Border.all(color: theme.dividerColor),
                ),
                child: Text(
                  '[${citation.id}] ${citation.source}',
                  style: TextStyle(
                    color: theme.colorScheme.primary,
                    fontSize: 12,
                  ),
                ),
              );
            }).toList(),
          ),
        ],
      ),
    );
  }

  Widget _buildAvatar({required bool isUser, required ThemeData theme}) {
    return Container(
      width: 32,
      height: 32,
      decoration: BoxDecoration(
        color: isUser
            ? theme.colorScheme.primary
            : theme.colorScheme.secondary,
        shape: BoxShape.circle,
      ),
      child: Icon(
        isUser ? Icons.person : Icons.smart_toy,
        size: 18,
        color: Colors.white,
      ),
    );
  }
}

class _LoadingIndicator extends StatelessWidget {
  const _LoadingIndicator();

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        _TypingDot(delay: 0),
        const SizedBox(width: 6),
        _TypingDot(delay: 200),
        const SizedBox(width: 6),
        _TypingDot(delay: 400),
      ],
    );
  }
}

class _TypingDot extends StatefulWidget {
  final int delay;

  const _TypingDot({required this.delay});

  @override
  State<_TypingDot> createState() => _TypingDotState();
}

class _TypingDotState extends State<_TypingDot>
    with SingleTickerProviderStateMixin {
  late AnimationController _controller;

  @override
  void initState() {
    super.initState();
    _controller = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 600),
    );
    Future.delayed(Duration(milliseconds: widget.delay), () {
      if (mounted) {
        _controller.repeat();
      }
    });
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: _controller,
      builder: (context, child) {
        final value = (_controller.value * 2 - 1).abs();
        return Transform.translate(
          offset: Offset(0, -6 * value),
          child: Opacity(
            opacity: 0.4 + 0.6 * (1 - value),
            child: Container(
              width: 8,
              height: 8,
              decoration: const BoxDecoration(
                color: Color(0xFF8b949e),
                shape: BoxShape.circle,
              ),
            ),
          ),
        );
      },
    );
  }
}

class _TypingIndicator extends StatelessWidget {
  const _TypingIndicator();

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 32),
      child: Row(
        children: [
          Container(
            width: 32,
            height: 32,
            decoration: const BoxDecoration(
              color: Color(0xFFa371f7),
              shape: BoxShape.circle,
            ),
            child: const Icon(Icons.smart_toy, size: 18, color: Colors.white),
          ),
          const SizedBox(width: 12),
          Container(
            padding: const EdgeInsets.all(20),
            decoration: BoxDecoration(
              color: const Color(0xFF161b22),
              borderRadius: BorderRadius.circular(12),
              border: Border.all(color: const Color(0xFF30363d)),
            ),
            child: const _LoadingIndicator(),
          ),
        ],
      ),
    );
  }
}

