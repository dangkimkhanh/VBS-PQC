import 'package:flutter/material.dart';

/// Colours shared with the web verification page (Tailwind slate/indigo/emerald).
class AppColors {
  static const primary = Color(0xFF4F46E5);
  // Navy of the PQC emblem; also the launcher icon background.
  static const navy = Color(0xFF001C46);
  static const background = Color(0xFFF8FAFC);
  static const muted = Color(0xFF64748B);
  static const success = Color(0xFF059669);
  static const danger = Color(0xFFE11D48);
  static const warning = Color(0xFFD97706);
}

ThemeData buildAppTheme() {
  return ThemeData(
    useMaterial3: true,
    colorScheme: ColorScheme.fromSeed(seedColor: AppColors.primary),
    scaffoldBackgroundColor: AppColors.background,
    appBarTheme: const AppBarTheme(
      backgroundColor: Colors.white,
      foregroundColor: Color(0xFF0F172A),
      elevation: 0,
      scrolledUnderElevation: 1,
    ),
  );
}
