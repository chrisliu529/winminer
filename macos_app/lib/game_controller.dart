import 'dart:async';

import 'package:flutter/foundation.dart';

import 'game_api.dart';
import 'game_models.dart';

class GameController extends ChangeNotifier {
  GameController(this._api);

  final GameApi _api;
  final List<GameLevel> levels = [];
  GameSnapshot? game;
  int selectedLevel = 1;
  bool busy = false;
  bool autoPlaying = false;
  String? errorMessage;

  Timer? _autoTimer;
  int _autoRun = 0;

  Future<void> initialize() async {
    if (busy || levels.isNotEmpty) return;
    busy = true;
    notifyListeners();
    try {
      levels
        ..clear()
        ..addAll(await _api.getLevels());
      if (levels.isEmpty) {
        throw StateError('No game levels are configured');
      }
      selectedLevel = levels.first.level;
      game = await _api.newGame(selectedLevel);
    } catch (error) {
      errorMessage = error.toString();
    } finally {
      busy = false;
      notifyListeners();
    }
  }

  void selectLevel(int level) {
    selectedLevel = level;
    notifyListeners();
  }

  Future<void> newGame() async {
    if (busy || levels.isEmpty) return;
    _stopAutoPlay();
    final oldGameId = game?.id;
    busy = true;
    errorMessage = null;
    notifyListeners();
    try {
      game = await _api.newGame(selectedLevel);
      if (oldGameId != null) {
        await _api.deleteGame(oldGameId);
      }
    } catch (error) {
      errorMessage = error.toString();
    } finally {
      busy = false;
      notifyListeners();
    }
  }

  Future<void> click(int x, int y) async {
    final current = game;
    if (current == null || current.status != GameStatus.playing) return;
    await _runAction(() => _api.click(current.id, x, y));
  }

  Future<void> toggleFlag(int x, int y) async {
    final current = game;
    if (current == null || current.status != GameStatus.playing) return;
    await _runAction(() => _api.toggleFlag(current.id, x, y));
  }

  Future<void> botStep() async {
    final current = game;
    if (current == null || current.status != GameStatus.playing) return;
    await _runAction(() => _api.botStep(current.id));
  }

  void toggleAutoPlay() {
    if (autoPlaying) {
      _stopAutoPlay();
      notifyListeners();
      return;
    }
    if (busy || game?.status != GameStatus.playing) return;
    autoPlaying = true;
    errorMessage = null;
    final run = ++_autoRun;
    notifyListeners();
    unawaited(_autoStep(run));
  }

  void pauseAutoPlay() {
    if (!autoPlaying) return;
    _stopAutoPlay();
    notifyListeners();
  }

  Future<void> _autoStep(int run) async {
    if (!autoPlaying || run != _autoRun) return;
    await botStep();
    if (run != _autoRun || !autoPlaying) return;
    if (errorMessage != null || game?.status != GameStatus.playing) {
      _stopAutoPlay();
      notifyListeners();
      return;
    }
    _autoTimer = Timer(
      const Duration(milliseconds: 180),
      () => unawaited(_autoStep(run)),
    );
  }

  Future<void> _runAction(Future<GameSnapshot> Function() action) async {
    if (busy) return;
    busy = true;
    errorMessage = null;
    notifyListeners();
    try {
      game = await action();
    } catch (error) {
      errorMessage = error.toString();
    } finally {
      busy = false;
      notifyListeners();
    }
  }

  void _stopAutoPlay() {
    autoPlaying = false;
    _autoRun++;
    _autoTimer?.cancel();
    _autoTimer = null;
  }

  @override
  void dispose() {
    _stopAutoPlay();
    super.dispose();
  }
}
