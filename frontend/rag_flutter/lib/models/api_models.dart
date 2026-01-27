class HealthResponse {
  final bool success;
  final HealthData? data;

  HealthResponse({required this.success, this.data});

  factory HealthResponse.fromJson(Map<String, dynamic> json) {
    return HealthResponse(
      success: json['success'] ?? false,
      data: json['data'] != null ? HealthData.fromJson(json['data']) : null,
    );
  }
}

class HealthData {
  final String status;
  final HealthComponents? components;

  HealthData({required this.status, this.components});

  factory HealthData.fromJson(Map<String, dynamic> json) {
    return HealthData(
      status: json['status'] ?? 'unknown',
      components: json['components'] != null
          ? HealthComponents.fromJson(json['components'])
          : null,
    );
  }
}

class HealthComponents {
  final String database;
  final String embedding;
  final String llm;

  HealthComponents({
    required this.database,
    required this.embedding,
    required this.llm,
  });

  factory HealthComponents.fromJson(Map<String, dynamic> json) {
    return HealthComponents(
      database: json['database'] ?? 'unknown',
      embedding: json['embedding'] ?? 'unknown',
      llm: json['llm'] ?? 'unknown',
    );
  }
}

class Conversation {
  final String id;
  final String title;
  final DateTime createdAt;
  final List<Message> messages;

  Conversation({
    required this.id,
    required this.title,
    required this.createdAt,
    required this.messages,
  });

  factory Conversation.fromJson(Map<String, dynamic> json) {
    return Conversation(
      id: json['id'] ?? '',
      title: json['title'] ?? 'New Chat',
      createdAt: json['created_at'] != null
          ? DateTime.parse(json['created_at'])
          : DateTime.now(),
      messages: (json['messages'] as List<dynamic>?)
              ?.map((m) => Message.fromJson(m))
              .toList() ??
          [],
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'title': title,
      'created_at': createdAt.toIso8601String(),
      'messages': messages.map((m) => m.toJson()).toList(),
    };
  }
}

class Message {
  final String role;
  final String content;
  final List<Citation>? citations;

  Message({
    required this.role,
    required this.content,
    this.citations,
  });

  factory Message.fromJson(Map<String, dynamic> json) {
    return Message(
      role: json['role'] ?? 'user',
      content: json['content'] ?? '',
      citations: json['citations'] != null
          ? (json['citations'] as List<dynamic>)
              .map((c) => Citation.fromJson(c))
              .toList()
          : null,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'role': role,
      'content': content,
      'citations': citations?.map((c) => c.toJson()).toList(),
    };
  }
}

class Citation {
  final int id;
  final String source;
  final String section;
  final String preview;

  Citation({
    required this.id,
    required this.source,
    required this.section,
    required this.preview,
  });

  factory Citation.fromJson(Map<String, dynamic> json) {
    return Citation(
      id: json['id'] ?? 0,
      source: json['source'] ?? '',
      section: json['section'] ?? '',
      preview: json['preview'] ?? '',
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'source': source,
      'section': section,
      'preview': preview,
    };
  }
}

class Document {
  final String id;
  final String source;
  final String? title;
  final String format;
  final String? createdAt;
  final String? updatedAt;
  final Map<String, String>? metadata;

  Document({
    required this.id,
    required this.source,
    this.title,
    required this.format,
    this.createdAt,
    this.updatedAt,
    this.metadata,
  });

  factory Document.fromJson(Map<String, dynamic> json) {
    return Document(
      id: json['id'] ?? '',
      source: json['source'] ?? '',
      title: json['title'],
      format: json['format'] ?? 'unknown',
      createdAt: json['created_at'],
      updatedAt: json['updated_at'],
      metadata: json['metadata'] != null
          ? Map<String, String>.from(json['metadata'])
          : null,
    );
  }
}

class UploadResponse {
  final bool success;
  final UploadData? data;

  UploadResponse({required this.success, this.data});

  factory UploadResponse.fromJson(Map<String, dynamic> json) {
    return UploadResponse(
      success: json['success'] ?? false,
      data: json['data'] != null ? UploadData.fromJson(json['data']) : null,
    );
  }
}

class UploadData {
  final String jobId;
  final String status;
  final String message;

  UploadData({
    required this.jobId,
    required this.status,
    required this.message,
  });

  factory UploadData.fromJson(Map<String, dynamic> json) {
    return UploadData(
      jobId: json['job_id'] ?? '',
      status: json['status'] ?? 'pending',
      message: json['message'] ?? '',
    );
  }
}

class ApiResponse<T> {
  final bool success;
  final T? data;
  final ApiError? error;

  ApiResponse({
    required this.success,
    this.data,
    this.error,
  });

  factory ApiResponse.fromJson(
    Map<String, dynamic> json,
    T Function(dynamic)? dataParser,
  ) {
    return ApiResponse(
      success: json['success'] ?? false,
      data: json['data'] != null && dataParser != null
          ? dataParser(json['data'])
          : null,
      error: json['error'] != null ? ApiError.fromJson(json['error']) : null,
    );
  }
}

class ApiError {
  final String code;
  final String message;

  ApiError({required this.code, required this.message});

  factory ApiError.fromJson(Map<String, dynamic> json) {
    return ApiError(
      code: json['code'] ?? 'UNKNOWN_ERROR',
      message: json['message'] ?? 'An error occurred',
    );
  }
}

