import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../providers/app_state.dart';
import '../providers/conversation_provider.dart';
import '../providers/document_provider.dart';
import '../widgets/conversation_sidebar.dart';
import '../widgets/chat_area.dart';
import '../widgets/document_sidebar.dart';

class HomeScreen extends StatelessWidget {
  const HomeScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: Row(
        children: [
          // Left sidebar - Conversations
          const ConversationSidebar(),
          
          // Main chat area
          const Expanded(
            child: ChatArea(),
          ),
          
          // Right sidebar - Documents
          const DocumentSidebar(),
        ],
      ),
    );
  }
}

