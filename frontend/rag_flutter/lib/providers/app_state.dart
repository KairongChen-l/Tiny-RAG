import 'package:flutter/foundation.dart';
import '../services/api_service.dart';
import '../models/api_models.dart';

class AppState extends ChangeNotifier {
  final ApiService _apiService = ApiService();
  
  bool _isApiOnline = false;
  bool _isEmbeddingOk = false;
  bool _isLlmOk = false;
  bool _isLoading = false;

  bool get isApiOnline => _isApiOnline;
  bool get isEmbeddingOk => _isEmbeddingOk;
  bool get isLlmOk => _isLlmOk;
  bool get isLoading => _isLoading;

  AppState() {
    checkHealth();
    // Check health every 30 seconds
    Future.delayed(const Duration(seconds: 30), () {
      checkHealth();
    });
  }

  Future<void> checkHealth() async {
    _isLoading = true;
    notifyListeners();

    try {
      final health = await _apiService.checkHealth();
      _isApiOnline = health.success && health.data?.status == 'healthy';
      
      if (health.data?.components != null) {
        _isEmbeddingOk = health.data!.components!.embedding == 'ok';
        _isLlmOk = health.data!.components!.llm == 'ok';
      }
    } catch (e) {
      _isApiOnline = false;
      _isEmbeddingOk = false;
      _isLlmOk = false;
    } finally {
      _isLoading = false;
      notifyListeners();
    }
  }
}

