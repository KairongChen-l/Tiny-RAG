import 'package:flutter/material.dart';
import 'package:file_picker/file_picker.dart';
import 'package:provider/provider.dart';
import '../providers/document_provider.dart';

class UploadModal extends StatefulWidget {
  final DocumentProvider documentProvider;

  const UploadModal({super.key, required this.documentProvider});

  @override
  State<UploadModal> createState() => _UploadModalState();
}

class _UploadModalState extends State<UploadModal> {
  bool _isUploading = false;
  String? _uploadStatus;

  @override
  Widget build(BuildContext context) {
    return Dialog(
      backgroundColor: const Color(0xFF161b22),
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(12),
        side: const BorderSide(color: Color(0xFF30363d)),
      ),
      child: Container(
        width: 500,
        padding: const EdgeInsets.all(20),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            // Header
            Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                const Text(
                  'Upload Document',
                  style: TextStyle(
                    fontSize: 18,
                    fontWeight: FontWeight.w600,
                    color: Color(0xFFe6edf3),
                  ),
                ),
                IconButton(
                  icon: const Icon(Icons.close),
                  color: const Color(0xFF8b949e),
                  onPressed: () => Navigator.pop(context),
                ),
              ],
            ),
            const SizedBox(height: 20),

            // Drop zone
            _buildDropZone(),

            if (_uploadStatus != null) ...[
              const SizedBox(height: 16),
              Text(
                _uploadStatus!,
                style: TextStyle(
                  color: _uploadStatus!.contains('Error') ||
                          _uploadStatus!.contains('Failed')
                      ? const Color(0xFFf85149)
                      : const Color(0xFF3fb950),
                  fontSize: 14,
                ),
                textAlign: TextAlign.center,
              ),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildDropZone() {
    return GestureDetector(
      onTap: _isUploading ? null : _pickFile,
      child: Container(
        padding: const EdgeInsets.all(40),
        decoration: BoxDecoration(
          border: Border.all(
            color: const Color(0xFF30363d),
            width: 2,
            style: BorderStyle.solid,
          ),
          borderRadius: BorderRadius.circular(8),
        ),
        child: Column(
          children: [
            Icon(
              _isUploading ? Icons.upload : Icons.folder,
              size: 48,
              color: const Color(0xFF8b949e),
            ),
            const SizedBox(height: 16),
            Text(
              _isUploading
                  ? 'Uploading...'
                  : 'Drop your file here or click to browse',
              style: const TextStyle(
                color: Color(0xFF8b949e),
                fontSize: 14,
              ),
            ),
            const SizedBox(height: 8),
            const Text(
              'Supports .md, .txt, .pdf',
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

  Future<void> _pickFile() async {
    final result = await FilePicker.platform.pickFiles(
      type: FileType.custom,
      allowedExtensions: ['md', 'txt', 'pdf'],
    );

    if (result != null && result.files.single.bytes != null) {
      await _uploadFile(result.files.single);
    }
  }

  Future<void> _uploadFile(PlatformFile file) async {
    setState(() {
      _isUploading = true;
      _uploadStatus = 'Uploading...';
    });

    try {
      final fileBytes = file.bytes!;
      final fileName = file.name;

      await widget.documentProvider.uploadDocument(
        fileBytes,
        fileName,
        {'title': fileName},
      );

      setState(() {
        _uploadStatus = 'Upload successful! Processing document...';
      });

      await Future.delayed(const Duration(seconds: 2));
      if (mounted) {
        Navigator.pop(context);
      }
    } catch (e) {
      setState(() {
        _uploadStatus = 'Error: $e';
      });
    } finally {
      setState(() {
        _isUploading = false;
      });
    }
  }
}

