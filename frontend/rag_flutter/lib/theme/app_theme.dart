import 'package:flutter/material.dart';

/// 应用主题配置
class AppTheme {
  /// 深色主题
  static ThemeData get darkTheme {
    return ThemeData(
      brightness: Brightness.dark,
      useMaterial3: false,
      primaryColor: const Color(0xFF58a6ff),
      scaffoldBackgroundColor: const Color(0xFF0d1117),
      colorScheme: const ColorScheme.dark(
        primary: Color(0xFF58a6ff),
        secondary: Color(0xFFa371f7),
        surface: Color(0xFF161b22),
        error: Color(0xFFf85149),
        onPrimary: Colors.white,
        onSurface: Color(0xFFe6edf3),
        onBackground: Color(0xFFe6edf3),
      ),
      cardColor: const Color(0xFF21262d),
      dividerColor: const Color(0xFF30363d),
      fontFamily: 'Plus Jakarta Sans',
      appBarTheme: const AppBarTheme(
        backgroundColor: Color(0xFF0d1117),
        elevation: 0,
        iconTheme: IconThemeData(color: Color(0xFF8b949e)),
      ),
      drawerTheme: const DrawerThemeData(
        backgroundColor: Color(0xFF161b22),
      ),
      inputDecorationTheme: InputDecorationTheme(
        filled: true,
        fillColor: const Color(0xFF21262d),
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: const BorderSide(color: Color(0xFF30363d)),
        ),
        enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: const BorderSide(color: Color(0xFF30363d)),
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: const BorderSide(color: Color(0xFF58a6ff)),
        ),
      ),
    );
  }

  /// 浅色主题
  static ThemeData get lightTheme {
    return ThemeData(
      brightness: Brightness.light,
      useMaterial3: false,
      primaryColor: const Color(0xFF0969da),
      scaffoldBackgroundColor: const Color(0xFFFFFFFF),
      colorScheme: const ColorScheme.light(
        primary: Color(0xFF0969da),
        secondary: Color(0xFF8250df),
        surface: Color(0xFFF6F8FA),
        error: Color(0xFFcf222e),
        onPrimary: Colors.white,
        onSurface: Color(0xFF24292f),
        onBackground: Color(0xFF24292f),
      ),
      cardColor: const Color(0xFFFFFFFF),
      dividerColor: const Color(0xFFd0d7de),
      fontFamily: 'Plus Jakarta Sans',
      appBarTheme: const AppBarTheme(
        backgroundColor: Color(0xFFFFFFFF),
        elevation: 0,
        iconTheme: IconThemeData(color: Color(0xFF57606a)),
      ),
      drawerTheme: const DrawerThemeData(
        backgroundColor: Color(0xFFF6F8FA),
      ),
      inputDecorationTheme: InputDecorationTheme(
        filled: true,
        fillColor: const Color(0xFFF6F8FA),
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: const BorderSide(color: Color(0xFFd0d7de)),
        ),
        enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: const BorderSide(color: Color(0xFFd0d7de)),
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: const BorderSide(color: Color(0xFF0969da)),
        ),
      ),
    );
  }

  /// 根据主题模式获取主题
  static ThemeData getTheme(bool isDark) {
    return isDark ? darkTheme : lightTheme;
  }
}

