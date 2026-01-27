import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../providers/document_provider.dart';
import '../models/api_models.dart';

class DocumentSidebar extends StatelessWidget {
  const DocumentSidebar({super.key});

  @override
  Widget build(BuildContext context) {
    final provider = Provider.of<DocumentProvider>(context);

    // Load documents on first build (only if not already loading)
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!provider.isLoading && provider.documents.isEmpty) {
        provider.loadDocuments();
      }
    });

    return Container(
      width: 300,
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
            child: const Row(
              children: [
                Icon(Icons.library_books, color: Color(0xFF58a6ff)),
                SizedBox(width: 8),
                Text(
                  'Uploaded Documents',
                  style: TextStyle(
                    fontSize: 16,
                    fontWeight: FontWeight.w600,
                    color: Color(0xFFe6edf3),
                  ),
                ),
              ],
            ),
          ),

          // Documents list
          Expanded(
            child: _buildDocumentsList(context, provider),
          ),
        ],
      ),
    );
  }

  Widget _buildDocumentsList(BuildContext context, DocumentProvider provider) {
    if (provider.isLoading && provider.documents.isEmpty) {
      return const Center(
        child: CircularProgressIndicator(),
      );
    }

    if (provider.documents.isEmpty) {
      return const Center(
        child: Padding(
          padding: EdgeInsets.all(24),
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Icon(Icons.folder_open, size: 48, color: Color(0xFF6e7681)),
              SizedBox(height: 16),
              Text(
                'No documents uploaded yet',
                textAlign: TextAlign.center,
                style: TextStyle(
                  color: Color(0xFF8b949e),
                  fontSize: 13,
                ),
              ),
              SizedBox(height: 8),
              Text(
                'Upload a document to get started',
                textAlign: TextAlign.center,
                style: TextStyle(
                  color: Color(0xFF6e7681),
                  fontSize: 12,
                ),
              ),
            ],
          ),
        ),
      );
    }

    return RefreshIndicator(
      onRefresh: () => provider.loadDocuments(),
      child: ListView.builder(
        padding: const EdgeInsets.all(12),
        itemCount: provider.documents.length,
        itemBuilder: (context, index) {
          final doc = provider.documents[index];
          return _DocumentItem(
            document: doc,
            onDelete: () => _deleteDocument(context, provider, doc.id),
          );
        },
      ),
    );
  }

  void _deleteDocument(
    BuildContext context,
    DocumentProvider provider,
    String id,
  ) {
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Delete Document'),
        content: const Text('Are you sure you want to delete this document?'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('Cancel'),
          ),
          TextButton(
            onPressed: () {
              provider.deleteDocument(id);
              Navigator.pop(context);
            },
            child: const Text('Delete', style: TextStyle(color: Colors.red)),
          ),
        ],
      ),
    );
  }
}

class _DocumentItem extends StatelessWidget {
  final Document document;
  final VoidCallback onDelete;

  const _DocumentItem({
    required this.document,
    required this.onDelete,
  });

  @override
  Widget build(BuildContext context) {
    final displayName = document.title ?? document.source ?? document.id;

    return Container(
      margin: const EdgeInsets.only(bottom: 8),
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: const Color(0xFF21262d),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: const Color(0xFF30363d)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Expanded(
                child: Text(
                  displayName,
                  style: const TextStyle(
                    fontSize: 14,
                    fontWeight: FontWeight.w500,
                    color: Color(0xFFe6edf3),
                  ),
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              IconButton(
                icon: const Icon(Icons.delete_outline, size: 18),
                color: const Color(0xFF8b949e),
                onPressed: onDelete,
              ),
            ],
          ),
          const SizedBox(height: 6),
          Row(
            children: [
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                decoration: BoxDecoration(
                  color: const Color(0xFF0d1117),
                  borderRadius: BorderRadius.circular(4),
                ),
                child: Text(
                  document.format.toUpperCase(),
                  style: const TextStyle(
                    fontSize: 11,
                    color: Color(0xFF8b949e),
                  ),
                ),
              ),
              if (document.createdAt != null) ...[
                const SizedBox(width: 12),
                Text(
                  document.createdAt!,
                  style: const TextStyle(
                    fontSize: 11,
                    color: Color(0xFF8b949e),
                  ),
                ),
              ],
            ],
          ),
        ],
      ),
    );
  }
}

