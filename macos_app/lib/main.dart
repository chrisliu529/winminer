import 'dart:io';

import 'package:material_ui/material_ui.dart';

import 'game_api.dart';
import 'game_screen.dart';

void main() {
  runApp(const WinminerApp());
}

class WinminerApp extends StatelessWidget {
  const WinminerApp({super.key});

  @override
  Widget build(BuildContext context) {
    final colorScheme = ColorScheme.fromSeed(
      seedColor: const Color(0xff315f8c),
      brightness: Brightness.light,
    );
    return MaterialApp(
      title: Platform.isIOS || Platform.isAndroid ? 'Minesweeper' : 'Winminer',
      theme: ThemeData(
        colorScheme: colorScheme,
        useMaterial3: true,
        scaffoldBackgroundColor: const Color(0xffedf1f5),
        cardTheme: const CardThemeData(elevation: 0, margin: EdgeInsets.zero),
      ),
      home: GameScreen(api: GoGameApi()),
    );
  }
}
