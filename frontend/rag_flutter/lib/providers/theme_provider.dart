import 'package:flutter/material.dart';
import 'package:shared_preferences/shared_preferences.dart';

/// 主题提供者 - 管理应用主题切换
class ThemeProvider extends ChangeNotifier {
  bool _isDarkMode = true;
  static const String _themeKey = 'is_dark_mode';

  bool get isDarkMode => _isDarkMode;

  ThemeProvider() {
    _loadTheme();
  }

  /// 从本地存储加载主题设置
  Future<void> _loadTheme() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      final savedTheme = prefs.getBool(_themeKey);
      if (savedTheme != null) {
        _isDarkMode = savedTheme;
        notifyListeners();
      }
    } catch (e) {
      // 如果加载失败，使用默认深色主题
      _isDarkMode = true;
    }
  }

  /// 切换主题
  Future<void> toggleTheme() async {
    _isDarkMode = !_isDarkMode;
    notifyListeners();
    await _saveTheme();
  }

  /// 设置主题
  Future<void> setDarkMode(bool isDark) async {
    if (_isDarkMode != isDark) {
      _isDarkMode = isDark;
      notifyListeners();
      await _saveTheme();
    }
  }

  /// 保存主题设置到本地存储
  Future<void> _saveTheme() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      await prefs.setBool(_themeKey, _isDarkMode);
    } catch (e) {
      // 保存失败不影响主题切换
    }
  }
}

