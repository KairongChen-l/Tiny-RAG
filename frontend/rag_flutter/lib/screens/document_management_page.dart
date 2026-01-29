import 'dart:io';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:file_picker/file_picker.dart';
import '../providers/document_provider.dart';
import '../models/api_models.dart';

class DocumentManagementPage extends StatefulWidget {
  const DocumentManagementPage({super.key});

  @override
  State<DocumentManagementPage> createState() => _DocumentManagementPageState();
}

class _DocumentManagementPageState extends State<DocumentManagementPage> {
  final Set<String> _selectedIds = {};
  bool _showDeleted = false;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final provider = Provider.of<DocumentProvider>(context);

    return Scaffold(
      appBar: AppBar(
        title: const Text('文档管理'),
        actions: [
          if (_selectedIds.isNotEmpty)
            IconButton(
              icon: const Icon(Icons.delete),
              onPressed: () => _showDeleteDialog(context, provider),
              tooltip: '删除选中',
            ),
          IconButton(
            icon: const Icon(Icons.refresh),
            onPressed: () => provider.loadDocuments(includeDeleted: _showDeleted),
            tooltip: '刷新',
          ),
          PopupMenuButton<String>(
            onSelected: (value) {
              if (value == 'toggle_deleted') {
                setState(() {
                  _showDeleted = !_showDeleted;
                });
                provider.loadDocuments(includeDeleted: _showDeleted);
              }
            },
            itemBuilder: (context) => [
              PopupMenuItem(
                value: 'toggle_deleted',
                child: Row(
                  children: [
                    Icon(_showDeleted ? Icons.visibility : Icons.visibility_off),
                    const SizedBox(width: 8),
                    Text(_showDeleted ? '隐藏已删除' : '显示已删除'),
                  ],
                ),
              ),
            ],
          ),
        ],
      ),
      body: Column(
        children: [
          // Stats bar
          if (provider.stats != null) _buildStatsBar(context, provider.stats!),
          
          // Document list
          Expanded(
            child: provider.isLoading && provider.documents.isEmpty
                ? const Center(child: CircularProgressIndicator())
                : provider.documents.isEmpty
                    ? Center(
                        child: Column(
                          mainAxisAlignment: MainAxisAlignment.center,
                          children: [
                            Icon(
                              Icons.description_outlined,
                              size: 64,
                              color: theme.colorScheme.onSurface.withOpacity(0.3),
                            ),
                            const SizedBox(height: 16),
                            Text(
                              '暂无文档',
                              style: theme.textTheme.titleLarge?.copyWith(
                                color: theme.colorScheme.onSurface.withOpacity(0.5),
                              ),
                            ),
                            const SizedBox(height: 8),
                            Text(
                              '点击右下角按钮上传文档',
                              style: theme.textTheme.bodyMedium?.copyWith(
                                color: theme.colorScheme.onSurface.withOpacity(0.5),
                              ),
                            ),
                          ],
                        ),
                      )
                    : ListView.builder(
                        itemCount: provider.documents.length,
                        itemBuilder: (context, index) {
                          final doc = provider.documents[index];
                          final isSelected = _selectedIds.contains(doc.id);
                          
                          return ListTile(
                            leading: Checkbox(
                              value: isSelected,
                              onChanged: (value) {
                                setState(() {
                                  if (value == true) {
                                    _selectedIds.add(doc.id);
                                  } else {
                                    _selectedIds.remove(doc.id);
                                  }
                                });
                              },
                            ),
                            title: Text(doc.title ?? doc.source),
                            subtitle: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Text('格式: ${doc.format}'),
                                if (doc.createdAt != null)
                                  Text('上传时间: ${doc.createdAt}'),
                              ],
                            ),
                            trailing: Row(
                              mainAxisSize: MainAxisSize.min,
                              children: [
                                IconButton(
                                  icon: const Icon(Icons.restore),
                                  onPressed: () => _restoreDocument(context, provider, doc.id),
                                  tooltip: '恢复',
                                ),
                                IconButton(
                                  icon: const Icon(Icons.delete),
                                  onPressed: () => _showDeleteSingleDialog(context, provider, doc.id),
                                  tooltip: '删除',
                                ),
                              ],
                            ),
                            onTap: () {
                              setState(() {
                                if (isSelected) {
                                  _selectedIds.remove(doc.id);
                                } else {
                                  _selectedIds.add(doc.id);
                                }
                              });
                            },
                          );
                        },
                      ),
          ),
        ],
      ),
      floatingActionButton: FloatingActionButton.extended(
        onPressed: () => _showUploadDialog(context, provider),
        icon: const Icon(Icons.upload_file),
        label: const Text('上传文档'),
      ),
      floatingActionButtonLocation: FloatingActionButtonLocation.endFloat,
    );
  }

  Widget _buildStatsBar(BuildContext context, Map<String, dynamic> stats) {
    final theme = Theme.of(context);
    
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: theme.colorScheme.surfaceVariant,
        border: Border(
          bottom: BorderSide(
            color: theme.dividerColor,
            width: 1,
          ),
        ),
      ),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceAround,
        children: [
          _buildStatItem(context, '总数', stats['total']?.toString() ?? '0'),
          _buildStatItem(context, '已处理', stats['processed']?.toString() ?? '0'),
          _buildStatItem(context, '处理中', stats['processing']?.toString() ?? '0'),
          _buildStatItem(context, '失败', stats['failed']?.toString() ?? '0'),
        ],
      ),
    );
  }

  Widget _buildStatItem(BuildContext context, String label, String value) {
    final theme = Theme.of(context);
    
    return Column(
      children: [
        Text(
          value,
          style: theme.textTheme.titleLarge?.copyWith(
            fontWeight: FontWeight.bold,
            color: theme.colorScheme.primary,
          ),
        ),
        const SizedBox(height: 4),
        Text(
          label,
          style: theme.textTheme.bodySmall?.copyWith(
            color: theme.colorScheme.onSurface.withOpacity(0.6),
          ),
        ),
      ],
    );
  }

  Future<void> _showUploadDialog(
    BuildContext context,
    DocumentProvider provider,
  ) async {
    final result = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (context) => const _UploadDialog(),
    );

    if (result != null && result['files'] != null) {
      final files = result['files'] as List<Map<String, dynamic>>;
      
      if (files.isEmpty) {
        if (context.mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            const SnackBar(content: Text('请先选择要上传的文件')),
          );
        }
        return;
      }

      // Show loading indicator
      if (context.mounted) {
        showDialog(
          context: context,
          barrierDismissible: false,
          builder: (context) => const Center(
            child: CircularProgressIndicator(),
          ),
        );
      }

      try {
        if (files.length == 1) {
          final file = files[0];
          final fileBytes = file['bytes'] as List<int>?;
          final fileName = file['fileName'] as String?;

          if (fileBytes == null || fileBytes.isEmpty) {
            throw Exception('文件内容为空');
          }
          if (fileName == null || fileName.isEmpty) {
            throw Exception('文件名无效');
          }

          await provider.uploadDocument(
            fileBytes,
            fileName,
            metadata: file['metadata'] as Map<String, String>?,
          );
          
          if (context.mounted) {
            Navigator.pop(context); // Close loading dialog
            ScaffoldMessenger.of(context).showSnackBar(
              const SnackBar(
                content: Text('文档上传成功'),
                backgroundColor: Colors.green,
              ),
            );
          }
        } else {
          // Validate all files before upload
          for (var file in files) {
            final fileBytes = file['bytes'] as List<int>?;
            final fileName = file['fileName'] as String?;
            if (fileBytes == null || fileBytes.isEmpty || fileName == null || fileName.isEmpty) {
              throw Exception('部分文件无效，请重新选择');
            }
          }

          await provider.batchUploadDocuments(files);
          
          if (context.mounted) {
            Navigator.pop(context); // Close loading dialog
            ScaffoldMessenger.of(context).showSnackBar(
              SnackBar(
                content: Text('成功上传 ${files.length} 个文档'),
                backgroundColor: Colors.green,
              ),
            );
          }
        }
      } catch (e) {
        if (context.mounted) {
          Navigator.pop(context); // Close loading dialog
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(
              content: Text('上传失败: $e'),
              backgroundColor: Colors.red,
              duration: const Duration(seconds: 5),
            ),
          );
        }
      }
    }
  }

  Future<void> _showDeleteDialog(
    BuildContext context,
    DocumentProvider provider,
  ) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('确认删除'),
        content: Text('确定要删除选中的 ${_selectedIds.length} 个文档吗？'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('取消'),
          ),
          TextButton(
            onPressed: () => Navigator.pop(context, true),
            child: const Text('删除'),
          ),
        ],
      ),
    );

    if (confirmed == true) {
      try {
        await provider.batchDeleteDocuments(_selectedIds.toList());
        setState(() {
          _selectedIds.clear();
        });
        
        if (context.mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            const SnackBar(content: Text('删除成功')),
          );
        }
      } catch (e) {
        if (context.mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: Text('删除失败: $e')),
          );
        }
      }
    }
  }

  Future<void> _showDeleteSingleDialog(
    BuildContext context,
    DocumentProvider provider,
    String id,
  ) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('确认删除'),
        content: const Text('确定要删除这个文档吗？'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('取消'),
          ),
          TextButton(
            onPressed: () => Navigator.pop(context, true),
            child: const Text('删除'),
          ),
        ],
      ),
    );

    if (confirmed == true) {
      try {
        await provider.deleteDocument(id);
        
        if (context.mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            const SnackBar(content: Text('删除成功')),
          );
        }
      } catch (e) {
        if (context.mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: Text('删除失败: $e')),
          );
        }
      }
    }
  }

  Future<void> _restoreDocument(
    BuildContext context,
    DocumentProvider provider,
    String id,
  ) async {
    try {
      await provider.restoreDocument(id);
      
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('恢复成功')),
        );
      }
    } catch (e) {
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('恢复失败: $e')),
        );
      }
    }
  }
}

class _UploadDialog extends StatefulWidget {
  const _UploadDialog();

  @override
  State<_UploadDialog> createState() => _UploadDialogState();
}

class _UploadDialogState extends State<_UploadDialog> {
  final List<Map<String, dynamic>> _selectedFiles = [];
  bool _isUploading = false;

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: const Text('上传文档'),
      content: SizedBox(
        width: double.maxFinite,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            if (_selectedFiles.isEmpty)
              const Text('请选择要上传的文件')
            else
              ..._selectedFiles.map((file) => ListTile(
                    leading: const Icon(Icons.description),
                    title: Text(file['fileName'] as String),
                    trailing: IconButton(
                      icon: const Icon(Icons.close),
                      onPressed: () {
                        setState(() {
                          _selectedFiles.remove(file);
                        });
                      },
                    ),
                  )),
            const SizedBox(height: 16),
            ElevatedButton.icon(
              onPressed: _pickFiles,
              icon: const Icon(Icons.add),
              label: const Text('选择文件'),
            ),
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: _isUploading ? null : () => Navigator.pop(context),
          child: const Text('取消'),
        ),
        ElevatedButton(
          onPressed: _isUploading || _selectedFiles.isEmpty
              ? null
              : () => Navigator.pop(context, {'files': _selectedFiles}),
          child: const Text('上传'),
        ),
      ],
    );
  }

  Future<void> _pickFiles() async {
    try {
      final result = await FilePicker.platform.pickFiles(
        allowMultiple: true,
        type: FileType.any,
      );

      if (result != null && result.files.isNotEmpty) {
        final files = <Map<String, dynamic>>[];
        
        for (var file in result.files) {
          List<int>? fileBytes;
          String fileName = file.name;

          // Try to read file bytes
          if (file.bytes != null) {
            // Web platform - bytes are already available
            fileBytes = file.bytes;
          } else if (file.path != null) {
            // Mobile/Desktop platform - read from file path
            try {
              final fileObj = File(file.path!);
              if (await fileObj.exists()) {
                fileBytes = await fileObj.readAsBytes();
              } else {
                if (mounted) {
                  ScaffoldMessenger.of(context).showSnackBar(
                    SnackBar(content: Text('文件不存在: $fileName')),
                  );
                }
                continue;
              }
            } catch (e) {
              if (mounted) {
                ScaffoldMessenger.of(context).showSnackBar(
                  SnackBar(content: Text('读取文件失败: $fileName - $e')),
                );
              }
              continue;
            }
          }

          if (fileBytes != null && fileBytes.isNotEmpty) {
            files.add({
              'bytes': fileBytes,
              'fileName': fileName,
              'metadata': null,
            });
          } else {
            if (mounted) {
              ScaffoldMessenger.of(context).showSnackBar(
                SnackBar(content: Text('文件为空或无法读取: $fileName')),
              );
            }
          }
        }
        
        if (files.isNotEmpty) {
          setState(() {
            _selectedFiles.addAll(files);
          });
        }
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('选择文件时出错: $e')),
        );
      }
    }
  }
}


