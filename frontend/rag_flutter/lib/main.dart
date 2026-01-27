import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'providers/app_state.dart';
import 'providers/conversation_provider.dart';
import 'providers/document_provider.dart';
import 'screens/home_screen.dart';

void main() {
  runApp(const RAGApp());
}

class RAGApp extends StatelessWidget {
  const RAGApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MultiProvider(
      providers: [
        ChangeNotifierProvider(create: (_) => AppState()),
        ChangeNotifierProvider(create: (_) => ConversationProvider()),
        ChangeNotifierProvider(create: (_) => DocumentProvider()),
      ],
      child: MaterialApp(
        title: 'RAG Knowledge Base',
        debugShowCheckedModeBanner: false,
        theme: ThemeData(
          brightness: Brightness.dark,
          primaryColor: const Color(0xFF58a6ff),
          scaffoldBackgroundColor: const Color(0xFF0d1117),
          colorScheme: const ColorScheme.dark(
            primary: Color(0xFF58a6ff),
            secondary: Color(0xFFa371f7),
            surface: Color(0xFF161b22),
            error: Color(0xFFf85149),
            onPrimary: Colors.white,
            onSurface: Color(0xFFe6edf3),
          ),
          cardColor: const Color(0xFF21262d),
          dividerColor: const Color(0xFF30363d),
          fontFamily: 'Plus Jakarta Sans',
        ),
        home: const HomeScreen(),
      ),
    );
  }
}

