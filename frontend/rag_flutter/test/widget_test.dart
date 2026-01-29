// This is a basic Flutter widget test.
//
// To perform an interaction with a widget in your test, use the WidgetTester
// utility in the flutter_test package. For example, you can send tap and scroll
// gestures. You can also use WidgetTester to find child widgets in the widget
// tree, read text, and verify that the values of widget properties are correct.

import 'package:flutter_test/flutter_test.dart';
import 'package:flutter/material.dart';

import 'package:rag_flutter/main.dart';

void main() {
  testWidgets('App renders chat UI smoke test', (WidgetTester tester) async {
    await tester.pumpWidget(const RAGApp());
    await tester.pumpAndSettle();

    // 顶部菜单按钮存在
    expect(find.byIcon(Icons.menu), findsOneWidget);

    // 输入框提示存在（非 loading 状态）
    expect(find.text('输入消息（Enter 发送，Shift+Enter 换行）'), findsOneWidget);
  });
}
