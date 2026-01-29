import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:provider/provider.dart';
import '../providers/conversation_provider.dart';

class _SendMessageIntent extends Intent {
  const _SendMessageIntent();
}

class _InsertNewlineIntent extends Intent {
  const _InsertNewlineIntent();
}

/// 聊天输入组件 - ChatGPT 风格的输入框
/// 
/// 特点：
/// - 初始单行，输入时自动增高
/// - Enter 发送，Shift+Enter 换行
/// - 发送时禁用输入
/// - 显示"正在生成"状态
class ChatInput extends StatefulWidget {
  const ChatInput({super.key});

  @override
  State<ChatInput> createState() => _ChatInputState();
}

class _ChatInputState extends State<ChatInput> {
  final TextEditingController _controller = TextEditingController();
  final FocusNode _focusNode = FocusNode();
  bool _isSending = false;
  int _lineCount = 1;
  static const int _maxLines = 6;

  @override
  void dispose() {
    _controller.dispose();
    _focusNode.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final provider = Provider.of<ConversationProvider>(context);
    final isLoading = provider.isLoading || _isSending;

    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        border: Border(
          top: BorderSide(color: theme.dividerColor, width: 1),
        ),
        color: theme.scaffoldBackgroundColor,
      ),
      child: SafeArea(
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.end,
          children: [
            // 输入框
            Expanded(
              child: Container(
                constraints: const BoxConstraints(maxHeight: 200),
                decoration: BoxDecoration(
                  color: theme.cardColor,
                  borderRadius: BorderRadius.circular(12),
                  border: Border.all(
                    color: _focusNode.hasFocus
                        ? theme.colorScheme.primary
                        : theme.dividerColor,
                    width: 1,
                  ),
                ),
                child: Shortcuts(
                  shortcuts: <ShortcutActivator, Intent>{
                    const SingleActivator(LogicalKeyboardKey.enter):
                        const _SendMessageIntent(),
                    const SingleActivator(LogicalKeyboardKey.enter, shift: true):
                        const _InsertNewlineIntent(),
                  },
                  child: Actions(
                    actions: <Type, Action<Intent>>{
                      _SendMessageIntent: CallbackAction<_SendMessageIntent>(
                        onInvoke: (intent) {
                          if (isLoading) return null;
                          final content = _controller.text.trim();
                          if (content.isEmpty) return null;
                          _handleSend(provider);
                          return null;
                        },
                      ),
                      _InsertNewlineIntent:
                          CallbackAction<_InsertNewlineIntent>(
                        onInvoke: (intent) {
                          if (isLoading) return null;
                          final text = _controller.text;
                          final selection = _controller.selection;
                          final insertAt = selection.isValid
                              ? selection.start
                              : text.length;
                          final newText =
                              text.replaceRange(insertAt, insertAt, '\n');
                          _controller.value = TextEditingValue(
                            text: newText,
                            selection: TextSelection.collapsed(
                              offset: insertAt + 1,
                            ),
                          );
                          _updateLineCount();
                          return null;
                        },
                      ),
                    },
                    child: TextField(
                      controller: _controller,
                      focusNode: _focusNode,
                      enabled: !isLoading,
                      maxLines: _maxLines,
                      minLines: 1,
                      keyboardType: TextInputType.multiline,
                      textInputAction: TextInputAction.newline,
                      style: TextStyle(
                        color: theme.colorScheme.onSurface,
                        fontSize: 15,
                        height: 1.5,
                      ),
                      decoration: InputDecoration(
                        hintText: isLoading
                            ? '正在生成回复...'
                            : '输入消息（Enter 发送，Shift+Enter 换行）',
                        hintStyle: TextStyle(
                          color: theme.colorScheme.onSurface.withOpacity(0.5),
                          fontSize: 15,
                        ),
                        border: InputBorder.none,
                        contentPadding: const EdgeInsets.symmetric(
                          horizontal: 16,
                          vertical: 12,
                        ),
                      ),
                      onChanged: (_) {
                        _updateLineCount();
                      },
                    ),
                  ),
                ),
              ),
            ),
            const SizedBox(width: 12),
            // 发送按钮
            _buildSendButton(context, provider, isLoading, theme),
          ],
        ),
      ),
    );
  }

  Widget _buildSendButton(
    BuildContext context,
    ConversationProvider provider,
    bool isLoading,
    ThemeData theme,
  ) {
    final hasText = _controller.text.trim().isNotEmpty;

    return Container(
      width: 40,
      height: 40,
      decoration: BoxDecoration(
        color: (hasText && !isLoading)
            ? theme.colorScheme.primary
            : theme.dividerColor,
        shape: BoxShape.circle,
      ),
      child: IconButton(
        icon: isLoading
            ? SizedBox(
                width: 20,
                height: 20,
                child: CircularProgressIndicator(
                  strokeWidth: 2,
                  valueColor: AlwaysStoppedAnimation<Color>(
                    theme.colorScheme.onSurface.withOpacity(0.5),
                  ),
                ),
              )
            : const Icon(Icons.send, size: 20),
        color: Colors.white,
        onPressed: (hasText && !isLoading)
            ? () => _handleSend(provider)
            : null,
        tooltip: '发送消息',
      ),
    );
  }

  void _updateLineCount() {
    final text = _controller.text;
    final lines = text.split('\n').length;
    if (lines != _lineCount) {
      setState(() {
        _lineCount = lines.clamp(1, _maxLines);
      });
    }
  }

  Future<void> _handleSend(ConversationProvider provider) async {
    final content = _controller.text.trim();
    if (content.isEmpty || _isSending) return;
    
    setState(() {
      _isSending = true;
    });

    _controller.clear();
    _focusNode.unfocus();

    try {
      await provider.sendMessage(content);
    } catch (e) {
      // 错误已在 ConversationProvider 中以 MessageStatus.error 形式落入对话流
    } finally {
      if (mounted) {
        setState(() {
          _isSending = false;
        });
        _focusNode.requestFocus();
      }
    }
  }
}

