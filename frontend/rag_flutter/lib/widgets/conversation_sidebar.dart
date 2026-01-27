import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../providers/conversation_provider.dart';
import '../providers/app_state.dart';

class ConversationSidebar extends StatelessWidget {
  const ConversationSidebar({super.key});

  @override
  Widget build(BuildContext context) {
    final conversationProvider = Provider.of<ConversationProvider>(context);
    final appState = Provider.of<AppState>(context);

    return Container(
      width: 280,
      color: const Color(0xFF161b22),
      child: Column(
        children: [
          // Header
          Container(
            padding: const EdgeInsets.all(20),
            decoration: const BoxDecoration(
              border: Border(
                bottom: BorderSide(color: Color(0xFF30363d), width: 1),
              ),
            ),
            child: Column(
              children: [
                const Row(
                  children: [
                    Icon(Icons.library_books, color: Color(0xFF58a6ff)),
                    SizedBox(width: 10),
                    Text(
                      'RAG Knowledge',
                      style: TextStyle(
                        fontSize: 18,
                        fontWeight: FontWeight.bold,
                        color: Color(0xFFe6edf3),
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 16),
                SizedBox(
                  width: double.infinity,
                  child: ElevatedButton.icon(
                    onPressed: () => conversationProvider.createConversation(),
                    icon: const Icon(Icons.add, size: 18),
                    label: const Text('New Chat'),
                    style: ElevatedButton.styleFrom(
                      backgroundColor: const Color(0xFF21262d),
                      foregroundColor: const Color(0xFFe6edf3),
                      side: const BorderSide(color: Color(0xFF30363d)),
                      padding: const EdgeInsets.symmetric(vertical: 12),
                    ),
                  ),
                ),
              ],
            ),
          ),

          // Conversations list
          Expanded(
            child: _buildConversationsList(context, conversationProvider),
          ),

          // Status bar
          Container(
            padding: const EdgeInsets.all(16),
            decoration: const BoxDecoration(
              border: Border(
                top: BorderSide(color: Color(0xFF30363d), width: 1),
              ),
            ),
            child: _buildStatusBar(context, appState),
          ),
        ],
      ),
    );
  }

  Widget _buildConversationsList(
    BuildContext context,
    ConversationProvider provider,
  ) {
    if (provider.isLoading && provider.conversations.isEmpty) {
      return const Center(child: CircularProgressIndicator());
    }

    if (provider.conversations.isEmpty) {
      return const Center(
        child: Text(
          'No conversations yet',
          style: TextStyle(color: Color(0xFF8b949e)),
        ),
      );
    }

    return ListView.builder(
      itemCount: provider.conversations.length,
      itemBuilder: (context, index) {
        final conv = provider.conversations[index];
        final isSelected = provider.currentConversation?.id == conv.id;

        return ListTile(
          selected: isSelected,
          selectedTileColor: const Color(0xFF21262d),
          title: Text(
            conv.title,
            style: const TextStyle(
              color: Color(0xFFe6edf3),
              fontSize: 14,
            ),
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
          ),
          trailing: isSelected
              ? IconButton(
                  icon: const Icon(Icons.delete_outline, size: 18),
                  color: const Color(0xFF8b949e),
                  onPressed: () => _deleteConversation(context, provider, conv.id),
                )
              : null,
          onTap: () => provider.selectConversation(conv.id),
        );
      },
    );
  }

  Widget _buildStatusBar(BuildContext context, AppState appState) {
    return Column(
      children: [
        _buildStatusItem('API', appState.isApiOnline),
        const SizedBox(height: 8),
        _buildStatusItem('Embedding', appState.isEmbeddingOk),
        const SizedBox(height: 8),
        _buildStatusItem('LLM', appState.isLlmOk),
      ],
    );
  }

  Widget _buildStatusItem(String label, bool isOnline) {
    return Row(
      children: [
        Container(
          width: 8,
          height: 8,
          decoration: BoxDecoration(
            color: isOnline ? const Color(0xFF3fb950) : const Color(0xFFf85149),
            shape: BoxShape.circle,
          ),
        ),
        const SizedBox(width: 6),
        Text(
          label,
          style: const TextStyle(
            fontSize: 12,
            color: Color(0xFF8b949e),
          ),
        ),
      ],
    );
  }

  void _deleteConversation(
    BuildContext context,
    ConversationProvider provider,
    String id,
  ) {
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Delete Conversation'),
        content: const Text('Are you sure you want to delete this conversation?'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('Cancel'),
          ),
          TextButton(
            onPressed: () {
              provider.deleteConversation(id);
              Navigator.pop(context);
            },
            child: const Text('Delete', style: TextStyle(color: Colors.red)),
          ),
        ],
      ),
    );
  }
}

