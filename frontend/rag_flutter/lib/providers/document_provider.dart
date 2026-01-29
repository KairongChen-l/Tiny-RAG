import 'package:flutter/foundation.dart';
import '../services/api_service.dart';
import '../models/api_models.dart';

// Import types
import '../models/api_models.dart' show JobStatus;

class DocumentProvider extends ChangeNotifier {
  final ApiService _apiService = ApiService();

  List<Document> _documents = [];
  bool _isLoading = false;
  String? _error;
  Map<String, dynamic>? _stats;

  List<Document> get documents => _documents;
  bool get isLoading => _isLoading;
  String? get error => _error;
  Map<String, dynamic>? get stats => _stats;

  DocumentProvider() {
    loadDocuments();
    loadStats();
  }

  Future<void> loadDocuments({bool includeDeleted = false}) async {
    _isLoading = true;
    _error = null;
    notifyListeners();

    try {
      _documents = await _apiService.getDocuments();
      _error = null;
    } catch (e) {
      _error = e.toString();
      _documents = [];
    } finally {
      _isLoading = false;
      notifyListeners();
    }
  }

  Future<void> loadStats() async {
    try {
      _stats = await _apiService.getDocumentStats();
      notifyListeners();
    } catch (e) {
      // Silently fail stats loading
      debugPrint('Failed to load stats: $e');
    }
  }

  Future<UploadResponse> uploadDocument(
    List<int> fileBytes,
    String fileName, {
    Map<String, String>? metadata,
  }) async {
    if (fileBytes.isEmpty) {
      _error = '文件内容为空';
      notifyListeners();
      throw Exception(_error);
    }

    if (fileName.isEmpty) {
      _error = '文件名无效';
      notifyListeners();
      throw Exception(_error);
    }

    _isLoading = true;
    _error = null;
    notifyListeners();

    try {
      final response = await _apiService.uploadDocument(
        fileBytes,
        fileName,
        metadata,
      );
      
      _error = null;
      
      // Reload documents after upload
      await loadDocuments();
      await loadStats();
      
      return response;
    } catch (e) {
      _error = e.toString();
      debugPrint('上传文档失败: $e');
      notifyListeners();
      rethrow;
    } finally {
      _isLoading = false;
      notifyListeners();
    }
  }

  Future<List<UploadResponse>> batchUploadDocuments(
    List<Map<String, dynamic>> files,
  ) async {
    _isLoading = true;
    _error = null;
    notifyListeners();

    try {
      final responses = await _apiService.batchUploadDocuments(files);
      
      // Reload documents after upload
      await loadDocuments();
      await loadStats();
      
      return responses;
    } catch (e) {
      _error = e.toString();
      notifyListeners();
      rethrow;
    } finally {
      _isLoading = false;
      notifyListeners();
    }
  }

  Future<void> deleteDocument(String id, {bool hardDelete = false}) async {
    _isLoading = true;
    _error = null;
    notifyListeners();

    try {
      await _apiService.deleteDocument(id, hardDelete: hardDelete);
      
      // Reload documents after delete
      await loadDocuments();
      await loadStats();
    } catch (e) {
      _error = e.toString();
      notifyListeners();
      rethrow;
    } finally {
      _isLoading = false;
      notifyListeners();
    }
  }

  Future<void> batchDeleteDocuments(
    List<String> documentIds, {
    bool hardDelete = false,
  }) async {
    _isLoading = true;
    _error = null;
    notifyListeners();

    try {
      await _apiService.batchDeleteDocuments(documentIds, hardDelete: hardDelete);
      
      // Reload documents after delete
      await loadDocuments();
      await loadStats();
    } catch (e) {
      _error = e.toString();
      notifyListeners();
      rethrow;
    } finally {
      _isLoading = false;
      notifyListeners();
    }
  }

  Future<void> restoreDocument(String id) async {
    _isLoading = true;
    _error = null;
    notifyListeners();

    try {
      await _apiService.restoreDocument(id);
      
      // Reload documents after restore
      await loadDocuments();
      await loadStats();
    } catch (e) {
      _error = e.toString();
      notifyListeners();
      rethrow;
    } finally {
      _isLoading = false;
      notifyListeners();
    }
  }

  Future<JobStatus> getJobStatus(String jobId) async {
    try {
      return await _apiService.getJobStatus(jobId);
    } catch (e) {
      throw Exception('Failed to get job status: $e');
    }
  }
}

