import 'package:flutter/material.dart';
import 'chat_page.dart';

/// 主屏幕 - 直接使用 ChatPage
/// 
/// 已重构为 ChatGPT 风格的单一页面体验
class HomeScreen extends StatelessWidget {
  const HomeScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return const ChatPage();
  }
}

