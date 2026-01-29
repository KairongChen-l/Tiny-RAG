import 'package:flutter/foundation.dart';
import '../services/api_service.dart';
import '../models/api_models.dart';

class ConversationProvider extends ChangeNotifier {
  final ApiService _apiService = ApiService();
  
  List<Conversation> _conversations = [];
  Conversation? _currentConversation;
  bool _isLoading = false;
  String? _error;

  List<Conversation> get conversations => _conversations;
  Conversation? get currentConversation => _currentConversation;
  bool get isLoading => _isLoading;
  String? get error => _error;

  Future<void> loadConversations() async {
    _isLoading = true;
    _error = null;
    notifyListeners();

    try {
      _conversations = await _apiService.getConversations();
      _error = null;
    } catch (e) {
      _error = e.toString();
    } finally {
      _isLoading = false;
      notifyListeners();
    }
  }

  Future<Conversation> createConversation({String? title}) async {
    _isLoading = true;
    _error = null;
    notifyListeners();

    try {
      final conversation = await _apiService.createConversation(title: title);
      _conversations.insert(0, conversation);
      _currentConversation = conversation;
      _error = null;
      return conversation;
    } catch (e) {
      _error = e.toString();
      rethrow;
    } finally {
      _isLoading = false;
      notifyListeners();
    }
  }

  Future<void> selectConversation(String id) async {
    _isLoading = true;
    _error = null;
    notifyListeners();

    try {
      _currentConversation = await _apiService.getConversation(id);
      _error = null;
    } catch (e) {
      _error = e.toString();
    } finally {
      _isLoading = false;
      notifyListeners();
    }
  }

  Future<void> deleteConversation(String id) async {
    _isLoading = true;
    _error = null;
    notifyListeners();

    try {
      await _apiService.deleteConversation(id);
      _conversations.removeWhere((c) => c.id == id);
      if (_currentConversation?.id == id) {
        _currentConversation = null;
      }
      _error = null;
    } catch (e) {
      _error = e.toString();
    } finally {
      _isLoading = false;
      notifyListeners();
    }
  }

  Future<Message> sendMessage(String content) async {
    if (_currentConversation == null) {
      await createConversation();
    }

    // 添加用户消息
    final userMessage = Message(role: 'user', content: content);
    _currentConversation!.messages.add(userMessage);
    
    // 添加加载中的助手消息
    final loadingMessage = Message(
      role: 'assistant',
      content: '',
      status: MessageStatus.loading,
    );
    _currentConversation!.messages.add(loadingMessage);
    
    _isLoading = true;
    _error = null;
    notifyListeners();

    try {
      final message = await _apiService.sendMessage(
        _currentConversation!.id,
        content,
      );
      
      // 替换加载消息为实际回复
      _currentConversation!.messages.removeLast();
      _currentConversation!.messages.add(message);
      
      // Update conversation title if it's the first message
      if (_currentConversation!.messages.length == 2) {
        final shortTitle = content.length > 30
            ? '${content.substring(0, 30)}...'
            : content;
        _currentConversation = Conversation(
          id: _currentConversation!.id,
          title: shortTitle,
          createdAt: _currentConversation!.createdAt,
          messages: _currentConversation!.messages,
        );
      }
      
      _error = null;
      await loadConversations();
      return message;
    } catch (e) {
      // 移除加载消息，添加错误消息
      _currentConversation!.messages.removeLast();
      final errorMessage = Message(
        role: 'assistant',
        content: '抱歉，发生了错误: ${e.toString()}',
        status: MessageStatus.error,
      );
      _currentConversation!.messages.add(errorMessage);
      _error = e.toString();
      notifyListeners();
      rethrow;
    } finally {
      _isLoading = false;
      notifyListeners();
    }
  }

  void clearCurrentConversation() {
    _currentConversation = null;
    notifyListeners();
  }
}

