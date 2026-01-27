import 'dart:convert';
import 'package:http/http.dart' as http;
import '../models/api_models.dart';

class ApiService {
  static const String baseUrl = 'http://localhost:8080/api/v1';
  
  final http.Client _client;

  ApiService({http.Client? client}) : _client = client ?? http.Client();

  // Health check
  Future<HealthResponse> checkHealth() async {
    try {
      final response = await _client.get(
        Uri.parse('$baseUrl/health'),
        headers: {'Content-Type': 'application/json'},
      );

      if (response.statusCode == 200) {
        return HealthResponse.fromJson(json.decode(response.body));
      } else {
        throw Exception('Health check failed: ${response.statusCode}');
      }
    } catch (e) {
      throw Exception('Failed to check health: $e');
    }
  }

  // Conversations
  Future<List<Conversation>> getConversations() async {
    try {
      final response = await _client.get(
        Uri.parse('$baseUrl/conversations'),
        headers: {'Content-Type': 'application/json'},
      );

      if (response.statusCode == 200) {
        final jsonData = json.decode(response.body);
        final data = jsonData['data'];
        if (data != null && data['conversations'] != null) {
          return (data['conversations'] as List<dynamic>)
              .map((c) => Conversation.fromJson(c))
              .toList();
        }
        return [];
      } else {
        throw Exception('Failed to get conversations: ${response.statusCode}');
      }
    } catch (e) {
      throw Exception('Failed to get conversations: $e');
    }
  }

  Future<Conversation> createConversation({String? title}) async {
    try {
      final response = await _client.post(
        Uri.parse('$baseUrl/conversations'),
        headers: {'Content-Type': 'application/json'},
        body: json.encode({'title': title ?? 'New Chat'}),
      );

      if (response.statusCode == 201 || response.statusCode == 200) {
        final jsonData = json.decode(response.body);
        final data = jsonData['data'] ?? jsonData;
        return Conversation(
          id: data['id'] ?? '',
          title: data['title'] ?? 'New Chat',
          createdAt: DateTime.now(),
          messages: [],
        );
      } else {
        throw Exception('Failed to create conversation: ${response.statusCode}');
      }
    } catch (e) {
      throw Exception('Failed to create conversation: $e');
    }
  }

  Future<Conversation> getConversation(String id) async {
    try {
      final response = await _client.get(
        Uri.parse('$baseUrl/conversations/$id'),
        headers: {'Content-Type': 'application/json'},
      );

      if (response.statusCode == 200) {
        final jsonData = json.decode(response.body);
        final data = jsonData['data'] ?? jsonData;
        return Conversation.fromJson(data);
      } else {
        throw Exception('Failed to get conversation: ${response.statusCode}');
      }
    } catch (e) {
      throw Exception('Failed to get conversation: $e');
    }
  }

  Future<void> deleteConversation(String id) async {
    try {
      final response = await _client.delete(
        Uri.parse('$baseUrl/conversations/$id'),
        headers: {'Content-Type': 'application/json'},
      );

      if (response.statusCode != 200 && response.statusCode != 204) {
        throw Exception('Failed to delete conversation: ${response.statusCode}');
      }
    } catch (e) {
      throw Exception('Failed to delete conversation: $e');
    }
  }

  Future<Message> sendMessage(String conversationId, String content) async {
    try {
      final response = await _client.post(
        Uri.parse('$baseUrl/conversations/$conversationId/messages'),
        headers: {'Content-Type': 'application/json'},
        body: json.encode({'content': content}),
      );

      if (response.statusCode == 200 || response.statusCode == 201) {
        final jsonData = json.decode(response.body);
        final data = jsonData['data'] ?? jsonData;
        
        // API returns { user_message: {...}, assistant_message: {...} }
        if (data['assistant_message'] != null) {
          final assistantMsg = data['assistant_message'];
          // Handle both direct Message format and wrapped format
          if (assistantMsg is Map) {
            return Message.fromJson(assistantMsg as Map<String, dynamic>);
          } else {
            throw Exception('Invalid assistant_message format: expected Map, got ${assistantMsg.runtimeType}');
          }
        } else if (data['content'] != null || data['role'] != null) {
          // Direct message format
          return Message.fromJson(data as Map<String, dynamic>);
        } else {
          throw Exception('Invalid response format: ${jsonData.keys}');
        }
      } else {
        final error = json.decode(response.body);
        throw Exception(error['error']?['message'] ?? 'Failed to send message');
      }
    } catch (e) {
      throw Exception('Failed to send message: $e');
    }
  }

  // Documents
  Future<List<Document>> getDocuments() async {
    try {
      final response = await _client.get(
        Uri.parse('$baseUrl/documents'),
        headers: {'Content-Type': 'application/json'},
      );

      if (response.statusCode == 200) {
        final jsonData = json.decode(response.body);
        final data = jsonData['data'];
        
        // Handle different response formats
        List<dynamic>? documents;
        if (data != null) {
          if (data is Map && data['documents'] != null) {
            documents = data['documents'] as List<dynamic>;
          } else if (data is List) {
            documents = data;
          }
        }
        
        if (documents != null) {
          return documents
              .map((d) => Document.fromJson(d as Map<String, dynamic>))
              .toList();
        }
        return [];
      } else {
        throw Exception('Failed to get documents: ${response.statusCode}');
      }
    } catch (e) {
      throw Exception('Failed to get documents: $e');
    }
  }

  Future<UploadResponse> uploadDocument(
    List<int> fileBytes,
    String fileName,
    Map<String, String>? metadata,
  ) async {
    try {
      final request = http.MultipartRequest(
        'POST',
        Uri.parse('$baseUrl/documents'),
      );

      request.files.add(
        http.MultipartFile.fromBytes(
          'file',
          fileBytes,
          filename: fileName,
        ),
      );

      if (metadata != null) {
        request.fields['metadata'] = json.encode(metadata);
      }

      final streamedResponse = await _client.send(request);
      final response = await http.Response.fromStream(streamedResponse);

      if (response.statusCode == 200 || response.statusCode == 202) {
        final jsonData = json.decode(response.body);
        // Handle both wrapped and unwrapped responses
        if (jsonData['data'] != null) {
          return UploadResponse.fromJson({
            'success': jsonData['success'] ?? true,
            'data': jsonData['data'],
          });
        } else {
          // Direct response format
          return UploadResponse.fromJson(jsonData);
        }
      } else {
        final error = json.decode(response.body);
        throw Exception(error['error']?['message'] ?? 'Upload failed');
      }
    } catch (e) {
      throw Exception('Failed to upload document: $e');
    }
  }

  Future<void> deleteDocument(String id) async {
    try {
      final response = await _client.delete(
        Uri.parse('$baseUrl/documents/$id'),
        headers: {'Content-Type': 'application/json'},
      );

      if (response.statusCode != 200 && response.statusCode != 204) {
        throw Exception('Failed to delete document: ${response.statusCode}');
      }
    } catch (e) {
      throw Exception('Failed to delete document: $e');
    }
  }

  // Query (single-turn)
  Future<Map<String, dynamic>> query(String query, {
    int? topK,
    Map<String, String>? filter,
    bool? enableRerank,
    String? provider,
  }) async {
    try {
      final body = <String, dynamic>{
        'query': query,
      };

      if (topK != null) body['options'] = {'top_k': topK};
      if (filter != null) {
        body['options'] ??= {};
        body['options']['filter'] = filter;
      }
      if (enableRerank != null) {
        body['options'] ??= {};
        body['options']['enable_rerank'] = enableRerank;
      }
      if (provider != null) {
        body['options'] ??= {};
        body['options']['provider'] = provider;
      }

      final response = await _client.post(
        Uri.parse('$baseUrl/query'),
        headers: {'Content-Type': 'application/json'},
        body: json.encode(body),
      );

      if (response.statusCode == 200) {
        return json.decode(response.body);
      } else {
        final error = json.decode(response.body);
        throw Exception(error['error']?['message'] ?? 'Query failed');
      }
    } catch (e) {
      throw Exception('Failed to query: $e');
    }
  }
}

