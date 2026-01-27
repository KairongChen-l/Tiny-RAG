import 'package:flutter/foundation.dart';
import '../services/api_service.dart';
import '../models/api_models.dart';

class DocumentProvider extends ChangeNotifier {
  final ApiService _apiService = ApiService();
  
  List<Document> _documents = [];
  bool _isLoading = false;
  String? _error;

  List<Document> get documents => _documents;
  bool get isLoading => _isLoading;
  String? get error => _error;

  Future<void> loadDocuments() async {
    _isLoading = true;
    _error = null;
    notifyListeners();

    try {
      _documents = await _apiService.getDocuments();
      _error = null;
    } catch (e) {
      _error = e.toString();
    } finally {
      _isLoading = false;
      notifyListeners();
    }
  }

  Future<UploadResponse> uploadDocument(
    List<int> fileBytes,
    String fileName,
    Map<String, String>? metadata,
  ) async {
    _isLoading = true;
    _error = null;
    notifyListeners();

    try {
      final response = await _apiService.uploadDocument(
        fileBytes,
        fileName,
        metadata,
      );
      
      // Reload documents after upload
      await Future.delayed(const Duration(seconds: 2));
      await loadDocuments();
      
      _error = null;
      return response;
    } catch (e) {
      _error = e.toString();
      rethrow;
    } finally {
      _isLoading = false;
      notifyListeners();
    }
  }

  Future<void> deleteDocument(String id) async {
    _isLoading = true;
    _error = null;
    notifyListeners();

    try {
      await _apiService.deleteDocument(id);
      _documents.removeWhere((d) => d.id == id);
      _error = null;
    } catch (e) {
      _error = e.toString();
    } finally {
      _isLoading = false;
      notifyListeners();
    }
  }
}

