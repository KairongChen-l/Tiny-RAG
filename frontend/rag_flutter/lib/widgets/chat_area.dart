import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:flutter_markdown/flutter_markdown.dart';
import '../providers/conversation_provider.dart';
import '../providers/document_provider.dart';
import '../models/api_models.dart';
import 'upload_modal.dart';

class ChatArea extends StatefulWidget {
  const ChatArea({super.key});

  @override
  State<ChatArea> createState() => _ChatAreaState();
}

class _ChatAreaState extends State<ChatArea> {
  final TextEditingController _inputController = TextEditingController();
  final ScrollController _scrollController = ScrollController();
  bool _isSending = false;

  @override
  void dispose() {
    _inputController.dispose();
    _scrollController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final conversationProvider = Provider.of<ConversationProvider>(context);
    final documentProvider = Provider.of<DocumentProvider>(context);

    return Column(
      children: [
        // Header
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 16),
          decoration: const BoxDecoration(
            border: Border(
              bottom: BorderSide(color: Color(0xFF30363d), width: 1),
            ),
          ),
          child: Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text(
                conversationProvider.currentConversation?.title ?? 'New Chat',
                style: const TextStyle(
                  fontSize: 16,
                  fontWeight: FontWeight.w600,
                  color: Color(0xFFe6edf3),
                ),
              ),
              ElevatedButton.icon(
                onPressed: () => _showUploadModal(context, documentProvider),
                icon: const Icon(Icons.upload_file, size: 18),
                label: const Text('Upload Document'),
                style: ElevatedButton.styleFrom(
                  backgroundColor: const Color(0xFF21262d),
                  foregroundColor: const Color(0xFFe6edf3),
                  side: const BorderSide(color: Color(0xFF30363d)),
                ),
              ),
            ],
          ),
        ),

        // Messages area
        Expanded(
          child: _buildMessagesArea(context, conversationProvider),
        ),

        // Input area
        Container(
          padding: const EdgeInsets.all(20),
          decoration: const BoxDecoration(
            border: Border(
              top: BorderSide(color: Color(0xFF30363d), width: 1),
            ),
            color: Color(0xFF161b22),
          ),
          child: _buildInputArea(context, conversationProvider),
        ),
      ],
    );
  }

  Widget _buildMessagesArea(
    BuildContext context,
    ConversationProvider provider,
  ) {
    final messages = provider.currentConversation?.messages ?? [];

    if (messages.isEmpty) {
      return const Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(Icons.chat_bubble_outline, size: 64, color: Color(0xFF8b949e)),
            SizedBox(height: 24),
            Text(
              'Welcome to RAG Knowledge Base',
              style: TextStyle(
                fontSize: 24,
                fontWeight: FontWeight.w600,
                color: Color(0xFFe6edf3),
              ),
            ),
            SizedBox(height: 8),
            Padding(
              padding: EdgeInsets.symmetric(horizontal: 24),
              child: Text(
                'Ask questions about your documents. Upload documents first, then start chatting to get answers with citations.',
                textAlign: TextAlign.center,
                style: TextStyle(
                  fontSize: 14,
                  color: Color(0xFF8b949e),
                ),
              ),
            ),
          ],
        ),
      );
    }

    return ListView.builder(
      controller: _scrollController,
      padding: const EdgeInsets.all(24),
      itemCount: messages.length + (provider.isLoading ? 1 : 0),
      itemBuilder: (context, index) {
        if (index == messages.length) {
          return const _TypingIndicator();
        }

        final message = messages[index];
        return _MessageBubble(message: message);
      },
    );
  }

  Widget _buildInputArea(
    BuildContext context,
    ConversationProvider provider,
  ) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.end,
      children: [
        Expanded(
          child: TextField(
            controller: _inputController,
            maxLines: null,
            minLines: 1,
            style: const TextStyle(color: Color(0xFFe6edf3)),
            decoration: InputDecoration(
              hintText: 'Ask a question about your documents...',
              hintStyle: const TextStyle(color: Color(0xFF6e7681)),
              filled: true,
              fillColor: const Color(0xFF21262d),
              border: OutlineInputBorder(
                borderRadius: BorderRadius.circular(12),
                borderSide: const BorderSide(color: Color(0xFF30363d)),
              ),
              enabledBorder: OutlineInputBorder(
                borderRadius: BorderRadius.circular(12),
                borderSide: const BorderSide(color: Color(0xFF30363d)),
              ),
              focusedBorder: OutlineInputBorder(
                borderRadius: BorderRadius.circular(12),
                borderSide: const BorderSide(color: Color(0xFF58a6ff)),
              ),
              contentPadding: const EdgeInsets.all(16),
            ),
            onSubmitted: (_) => _sendMessage(context, provider),
          ),
        ),
        const SizedBox(width: 12),
        IconButton(
          onPressed: _isSending || provider.isLoading
              ? null
              : () => _sendMessage(context, provider),
          icon: const Icon(Icons.send),
          style: IconButton.styleFrom(
            backgroundColor: const Color(0xFF58a6ff),
            foregroundColor: Colors.white,
            padding: const EdgeInsets.all(12),
          ),
        ),
      ],
    );
  }

  Future<void> _sendMessage(
    BuildContext context,
    ConversationProvider provider,
  ) async {
    final content = _inputController.text.trim();
    if (content.isEmpty || _isSending) return;

    setState(() {
      _isSending = true;
    });
    _inputController.clear();

    try {
      await provider.sendMessage(content);
      // Wait for UI to update before scrolling
      await Future.delayed(const Duration(milliseconds: 100));
      if (_scrollController.hasClients) {
        _scrollController.animateTo(
          _scrollController.position.maxScrollExtent,
          duration: const Duration(milliseconds: 300),
          curve: Curves.easeOut,
        );
      }
    } catch (e) {
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Error: $e')),
        );
      }
    } finally {
      if (mounted) {
        setState(() {
          _isSending = false;
        });
      }
    }
  }

  void _showUploadModal(
    BuildContext context,
    DocumentProvider provider,
  ) {
    showDialog(
      context: context,
      builder: (context) => UploadModal(documentProvider: provider),
    );
  }
}

class _MessageBubble extends StatelessWidget {
  final Message message;

  const _MessageBubble({required this.message});

  @override
  Widget build(BuildContext context) {
    final isUser = message.role == 'user';

    return Padding(
      padding: const EdgeInsets.only(bottom: 24),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisAlignment:
            isUser ? MainAxisAlignment.end : MainAxisAlignment.start,
        children: [
          if (!isUser) ...[
            CircleAvatar(
              radius: 16,
              backgroundColor: const Color(0xFFa371f7),
              child: const Icon(Icons.smart_toy, size: 18, color: Colors.white),
            ),
            const SizedBox(width: 12),
          ],
          Flexible(
            child: Container(
              padding: const EdgeInsets.all(20),
              decoration: BoxDecoration(
                color: isUser
                    ? const Color(0xFF21262d)
                    : const Color(0xFF161b22),
                borderRadius: BorderRadius.circular(12),
                border: Border.all(color: const Color(0xFF30363d)),
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  MarkdownBody(
                    data: message.content,
                    styleSheet: MarkdownStyleSheet(
                      p: const TextStyle(
                        color: Color(0xFFe6edf3),
                        fontSize: 15,
                        height: 1.7,
                      ),
                      code: const TextStyle(
                        backgroundColor: Color(0xFF1f2428),
                        color: Color(0xFFe6edf3),
                      ),
                    ),
                  ),
                  if (message.citations != null && message.citations!.isNotEmpty)
                    _buildCitations(message.citations!),
                ],
              ),
            ),
          ),
          if (isUser) ...[
            const SizedBox(width: 12),
            const CircleAvatar(
              radius: 16,
              backgroundColor: Color(0xFF58a6ff),
              child: Icon(Icons.person, size: 18, color: Colors.white),
            ),
          ],
        ],
      ),
    );
  }

  Widget _buildCitations(List<Citation> citations) {
    return Padding(
      padding: const EdgeInsets.only(top: 16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text(
            'SOURCES',
            style: TextStyle(
              fontSize: 12,
              fontWeight: FontWeight.w600,
              color: Color(0xFF8b949e),
              letterSpacing: 0.5,
            ),
          ),
          const SizedBox(height: 8),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: citations.map((citation) {
              return Chip(
                label: Text('[${citation.id}] ${citation.source}'),
                backgroundColor: const Color(0xFF21262d),
                side: const BorderSide(color: Color(0xFF30363d)),
                labelStyle: const TextStyle(
                  color: Color(0xFF58a6ff),
                  fontSize: 12,
                ),
              );
            }).toList(),
          ),
        ],
      ),
    );
  }
}

class _TypingIndicator extends StatelessWidget {
  const _TypingIndicator();

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 24),
      child: Row(
        children: [
          const CircleAvatar(
            radius: 16,
            backgroundColor: Color(0xFFa371f7),
            child: Icon(Icons.smart_toy, size: 18, color: Colors.white),
          ),
          const SizedBox(width: 12),
          Container(
            padding: const EdgeInsets.all(16),
            decoration: BoxDecoration(
              color: const Color(0xFF161b22),
              borderRadius: BorderRadius.circular(12),
              border: Border.all(color: const Color(0xFF30363d)),
            ),
            child: const Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                _TypingDot(delay: 0),
                SizedBox(width: 4),
                _TypingDot(delay: 200),
                SizedBox(width: 4),
                _TypingDot(delay: 400),
              ],
            ),
          ),
        ],
      ),
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
    )..repeat();
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
          offset: Offset(0, -8 * value),
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

