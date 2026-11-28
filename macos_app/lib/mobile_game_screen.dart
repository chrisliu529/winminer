import 'dart:async';
import 'dart:math' as math;

import 'package:flutter/gestures.dart';
import 'package:flutter/services.dart';
import 'package:material_ui/material_ui.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'game_controller.dart';
import 'game_models.dart';

class MobileGameScreen extends StatefulWidget {
  const MobileGameScreen({
    super.key,
    required this.controller,
    required this.onActionSound,
    this.now,
  });

  final GameController controller;
  final void Function(ActionSound sound) onActionSound;
  final DateTime Function()? now;

  @override
  State<MobileGameScreen> createState() => _MobileGameScreenState();
}

class _MobileGameScreenState extends State<MobileGameScreen>
    with WidgetsBindingObserver, SingleTickerProviderStateMixin {
  static const _levelNames = ['初级', '中级', '高级'];
  static String _bestTimeKey(int level) => 'mobile.best_time.v1.$level';

  final ValueNotifier<int> _elapsed = ValueNotifier(0);
  final ValueNotifier<(int, GameStatus)?> _summary = ValueNotifier(null);
  final Map<int, int> _bestTimes = {};
  SharedPreferencesAsync? _preferences;
  late Future<void> _bestTimesTask;
  bool _loadingBestTimes = true;
  bool _bestTimesUnavailable = false;
  Timer? _clock;
  Timer? _idleTimer;
  DateTime? _startedAt;
  DateTime? _idleDeadline;
  String? _gameId;
  GameStatus? _gameStatus;
  bool _showSolver = false;
  bool _flagMode = false;
  GameHint? _highlight;
  late final AnimationController _highlightAnimation;
  DateTime get _now => widget.now?.call() ?? DateTime.now();

  @override
  void initState() {
    super.initState();
    _highlightAnimation =
        AnimationController(vsync: this, duration: const Duration(seconds: 1))
          ..addStatusListener((status) {
            if (status == AnimationStatus.completed && mounted) {
              setState(() => _highlight = null);
            }
          });
    WidgetsBinding.instance.addObserver(this);
    widget.controller.gameState.addListener(_gameChanged);
    _gameChanged();
    _bestTimesTask = _loadBestTimes();
  }

  Future<void> _loadBestTimes() async {
    try {
      final preferences = SharedPreferencesAsync();
      final values = await preferences.getAll(
        allowList: {
          for (var level = 1; level <= _levelNames.length; level++)
            _bestTimeKey(level),
        },
      );
      final loaded = <int, int>{};
      for (var level = 1; level <= _levelNames.length; level++) {
        final value = values[_bestTimeKey(level)];
        if (value != null) {
          if (value is! int || value < 0) {
            throw FormatException('Invalid best time for level $level');
          }
          loaded[level] = value;
        }
      }
      _bestTimes.addAll(loaded);
      _preferences = preferences;
    } on PlatformException catch (error) {
      _reportBestTimeError(error, unavailable: true);
    } on MissingPluginException catch (error) {
      _reportBestTimeError(error, unavailable: true);
    } on StateError catch (error) {
      _reportBestTimeError(error, unavailable: true);
    } on FormatException catch (error) {
      _reportBestTimeError(error, unavailable: true);
    } finally {
      _loadingBestTimes = false;
      if (mounted) setState(() {});
    }
  }

  void _recordBestTime(int level, int seconds) {
    _bestTimesTask = _bestTimesTask.then((_) async {
      final preferences = _preferences;
      if (preferences == null) return;
      final previous = _bestTimes[level];
      if (previous != null && previous <= seconds) return;
      try {
        await preferences.setInt(_bestTimeKey(level), seconds);
        _bestTimes[level] = seconds;
        if (mounted) setState(() {});
      } on PlatformException catch (error) {
        _reportBestTimeError(error);
      } on MissingPluginException catch (error) {
        _reportBestTimeError(error);
      } on StateError catch (error) {
        _reportBestTimeError(error);
      }
    });
  }

  void _reportBestTimeError(Object error, {bool unavailable = false}) {
    if (unavailable) _bestTimesUnavailable = true;
    debugPrint('Unable to store best times: $error');
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      ScaffoldMessenger.maybeOf(context)
          ?.showSnackBar(SnackBar(content: Text('无法保存或读取最佳成绩：$error')));
    });
  }

  void _gameChanged() {
    final game = widget.controller.game;
    final justLost =
        game != null &&
        game.id == _gameId &&
        _gameStatus == GameStatus.playing &&
        game.status == GameStatus.lost;
    _gameStatus = game?.status;
    if (justLost) widget.onActionSound(ActionSound.mine);
    _summary.value = game == null ? null : (game.remainingMines, game.status);
    if (game?.id != _gameId) {
      _gameId = game?.id;
      _clock?.cancel();
      _idleTimer?.cancel();
      _startedAt = null;
      _idleDeadline = null;
      _highlightAnimation.stop();
      _highlight = null;
      _elapsed.value = 0;
      if (mounted) setState(() => _showSolver = false);
    }
    if (game != null && game.status != GameStatus.playing) {
      _updateElapsed();
      _clock?.cancel();
      _idleTimer?.cancel();
      _idleDeadline = null;
    }
  }

  void _updateElapsed() {
    final start = _startedAt;
    if (start != null) {
      _elapsed.value = math.max(0, _now.difference(start).inSeconds);
    }
  }

  void _scheduleSolver() {
    if (_showSolver ||
        _startedAt == null ||
        widget.controller.game?.status != GameStatus.playing) {
      return;
    }
    _idleTimer?.cancel();
    _idleDeadline = _now.add(const Duration(seconds: 5));
    _idleTimer = Timer(const Duration(seconds: 5), _showSolverAfterIdle);
  }

  void _showSolverAfterIdle() {
    if (!mounted || widget.controller.game?.status != GameStatus.playing) {
      return;
    }
    _idleDeadline = null;
    setState(() => _showSolver = true);
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state != AppLifecycleState.resumed) return;
    if (widget.controller.game?.status != GameStatus.playing) return;
    _updateElapsed();
    final deadline = _idleDeadline;
    if (deadline != null && !_now.isBefore(deadline)) {
      _idleTimer?.cancel();
      _showSolverAfterIdle();
    }
  }

  Future<void> _act(int x, int y, {required bool flag}) async {
    final controller = widget.controller;
    final before = controller.game;
    if (before == null ||
        controller.busy ||
        before.status != GameStatus.playing) {
      return;
    }
    final cell = before.cells[y * before.columns + x];
    if (cell.state != CellState.unknown &&
        !(flag && cell.state == CellState.flag)) {
      return;
    }
    _clearHighlight();
    final actionTime = _now;
    final level = controller.selectedLevel;
    if (flag) {
      await controller.toggleFlag(x, y);
    } else {
      await controller.click(x, y);
    }
    if (!mounted) return;
    final after = controller.game;
    if (after == null || identical(before, after) || after.id != before.id) {
      return;
    }
    if (after.status != GameStatus.lost) {
      widget.onActionSound(flag ? ActionSound.flag : ActionSound.reveal);
    }
    if (!flag && _startedAt == null) {
      _startedAt = actionTime;
      _updateElapsed();
      if (after.status == GameStatus.playing) {
        _clock = Timer.periodic(
          const Duration(seconds: 1),
          (_) => _updateElapsed(),
        );
      }
    }
    if (after.status == GameStatus.won && _startedAt != null) {
      _updateElapsed();
      _recordBestTime(level, _elapsed.value);
    }
    if (after.status == GameStatus.playing) _scheduleSolver();
  }

  void _clearHighlight() {
    _highlightAnimation.stop();
    if (_highlight != null) setState(() => _highlight = null);
  }

  Future<void> _solve() async {
    final game = widget.controller.game;
    _clearHighlight();
    final hint = await widget.controller.hint();
    if (!mounted || hint == null || !identical(widget.controller.game, game)) {
      return;
    }
    setState(() => _highlight = hint);
    _highlightAnimation.forward(from: 0);
  }

  Future<void> _newGame([int? level]) async {
    if (widget.controller.busy) return;
    final oldLevel = widget.controller.selectedLevel;
    final oldGameId = widget.controller.game?.id;
    if (level != null) widget.controller.selectLevel(level);
    await widget.controller.newGame();
    if (!mounted) return;
    if (level != null && widget.controller.game?.id == oldGameId) {
      widget.controller.selectLevel(oldLevel);
    }
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    widget.controller.gameState.removeListener(_gameChanged);
    _clock?.cancel();
    _idleTimer?.cancel();
    _highlightAnimation.dispose();
    _elapsed.dispose();
    _summary.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return SafeArea(
      child: Padding(
        padding: const EdgeInsets.all(10),
        child: DecoratedBox(
          decoration: _bevel(const Color(0xffc0c0c0)),
          child: Padding(
            padding: const EdgeInsets.all(10),
            child: Column(
              children: [
                ValueListenableBuilder<(int, GameStatus)?>(
                  valueListenable: _summary,
                  builder: (context, summary, _) => Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      _Led(value: summary?.$1 ?? 0),
                      _BevelButton(
                        key: const ValueKey('restart'),
                        onTap: () => _newGame(),
                        child: Semantics(
                          label: switch (summary?.$2) {
                            GameStatus.won => '获胜，重新开始',
                            GameStatus.lost => '失败，重新开始',
                            _ => '重新开始',
                          },
                          child: CustomPaint(
                            size: const Size(28, 28),
                            painter: _FacePainter(summary?.$2),
                          ),
                        ),
                      ),
                      Column(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          ValueListenableBuilder<int>(
                            valueListenable: _elapsed,
                            builder: (context, seconds, _) => _Led(
                              key: const ValueKey('timer'),
                              value: seconds,
                            ),
                          ),
                          const SizedBox(height: 2),
                          AnimatedBuilder(
                            animation: widget.controller,
                            builder: (context, _) {
                              final best =
                                  _bestTimes[widget.controller.selectedLevel];
                              final value = _loadingBestTimes
                                  ? '读取中'
                                  : _bestTimesUnavailable
                                  ? '不可用'
                                  : best == null
                                  ? '--'
                                  : '$best 秒';
                              return SizedBox(
                                width: 76,
                                child: FittedBox(
                                  fit: BoxFit.scaleDown,
                                  child: Text(
                                    '最佳：$value',
                                    key: const ValueKey('best-time'),
                                    style: const TextStyle(fontSize: 13),
                                  ),
                                ),
                              );
                            },
                          ),
                        ],
                      ),
                    ],
                  ),
                ),
                AnimatedBuilder(
                  animation: widget.controller,
                  builder: (context, _) =>
                      widget.controller.errorMessage == null
                      ? const SizedBox.shrink()
                      : Text(
                          widget.controller.errorMessage!,
                          style: const TextStyle(color: Colors.red),
                        ),
                ),
                Expanded(
                  child: Center(
                    child: _PhoneBoard(
                      controller: widget.controller,
                      flagMode: _flagMode,
                      onAction: _act,
                      highlight: _highlight,
                      highlightAnimation: _highlightAnimation,
                    ),
                  ),
                ),
                AnimatedBuilder(
                  animation: widget.controller,
                  builder: (context, _) => Row(
                    children: [
                      for (final (index, label) in _levelNames.indexed)
                        Padding(
                          padding: const EdgeInsets.only(right: 2),
                          child: _BevelButton(
                            key: ValueKey('level-${index + 1}'),
                            width: 48,
                            selected:
                                widget.controller.selectedLevel == index + 1,
                            onTap:
                                widget.controller.busy ||
                                    widget.controller.selectedLevel == index + 1
                                ? null
                                : () => _newGame(index + 1),
                            child: Text(label),
                          ),
                        ),
                      const Spacer(),
                      if (_showSolver)
                        _BevelButton(
                          key: const ValueKey('bot-step'),
                          width: 48,
                          onTap:
                              widget.controller.busy ||
                                  widget.controller.game?.status !=
                                      GameStatus.playing
                              ? null
                              : _solve,
                          child: const Text('提示'),
                        )
                      else
                        const SizedBox(width: 48),
                      const SizedBox(width: 4),
                      _BevelButton(
                        key: const ValueKey('flag-mode'),
                        width: 48,
                        selected: _flagMode,
                        onTap: () => setState(() => _flagMode = !_flagMode),
                        child: Text(_flagMode ? '插旗' : '挖开'),
                      ),
                    ],
                  ),
                ),
                const SizedBox(height: 8),
                Align(
                  alignment: Alignment.centerLeft,
                  child: Text(
                    _flagMode ? '轻点插旗 · 长按插旗' : '轻点挖开 · 长按插旗',
                    style: const TextStyle(fontSize: 12),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

BoxDecoration _bevel(Color color, {bool pressed = false}) => BoxDecoration(
  color: color,
  border: Border(
    top: BorderSide(
      color: pressed ? Colors.grey.shade700 : Colors.white,
      width: 3,
    ),
    left: BorderSide(
      color: pressed ? Colors.grey.shade700 : Colors.white,
      width: 3,
    ),
    right: BorderSide(
      color: pressed ? Colors.white : Colors.grey.shade700,
      width: 3,
    ),
    bottom: BorderSide(
      color: pressed ? Colors.white : Colors.grey.shade700,
      width: 3,
    ),
  ),
);

class _Led extends StatelessWidget {
  const _Led({super.key, required this.value});

  final int value;

  @override
  Widget build(BuildContext context) {
    final digits = value.clamp(0, 999).toString().padLeft(3, '0');
    return Semantics(
      label: digits,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 5, vertical: 3),
        decoration: _bevel(Colors.black, pressed: true),
        child: CustomPaint(
          size: const Size(60, 30),
          painter: _LedPainter(digits),
        ),
      ),
    );
  }
}

class _FacePainter extends CustomPainter {
  const _FacePainter(this.status);

  final GameStatus? status;

  @override
  void paint(Canvas canvas, Size size) {
    canvas.scale(size.width / 28, size.height / 28);
    final ink = Paint()
      ..color = const Color(0xff3d2c19)
      ..strokeWidth = 1.7
      ..strokeCap = StrokeCap.round;
    canvas.drawCircle(
      const Offset(14, 14),
      13,
      Paint()..color = const Color(0xffffd446),
    );
    canvas.drawCircle(
      const Offset(14, 14),
      13,
      ink..style = PaintingStyle.stroke,
    );
    ink.style = PaintingStyle.fill;
    if (status == GameStatus.won) {
      canvas.drawRRect(
        RRect.fromRectAndRadius(
          const Rect.fromLTWH(4, 8, 9, 6),
          const Radius.circular(2),
        ),
        ink,
      );
      canvas.drawRRect(
        RRect.fromRectAndRadius(
          const Rect.fromLTWH(15, 8, 9, 6),
          const Radius.circular(2),
        ),
        ink,
      );
      canvas.drawLine(const Offset(13, 10), const Offset(15, 10), ink);
    } else if (status == GameStatus.lost) {
      for (final x in [9.0, 19.0]) {
        canvas.drawLine(Offset(x - 2, 8), Offset(x + 2, 12), ink);
        canvas.drawLine(Offset(x + 2, 8), Offset(x - 2, 12), ink);
      }
    } else {
      canvas.drawCircle(const Offset(9, 10), 1.5, ink);
      canvas.drawCircle(const Offset(19, 10), 1.5, ink);
    }
    ink.style = PaintingStyle.stroke;
    if (status == GameStatus.lost) {
      canvas.drawOval(const Rect.fromLTWH(11, 17, 6, 5), ink);
    } else {
      canvas.drawArc(
        const Rect.fromLTWH(7, 11, 14, 11),
        0.2,
        math.pi - 0.4,
        false,
        ink,
      );
    }
  }

  @override
  bool shouldRepaint(covariant _FacePainter oldDelegate) =>
      oldDelegate.status != status;
}

class _BombPainter extends CustomPainter {
  const _BombPainter();

  @override
  void paint(Canvas canvas, Size size) {
    final ink = Paint()..color = const Color(0xff252525);
    final center = Offset(size.width * .45, size.height * .58);
    canvas.drawCircle(center, size.width * .31, ink);
    canvas.drawLine(
      Offset(size.width * .6, size.height * .32),
      Offset(size.width * .78, size.height * .13),
      ink..strokeWidth = size.width * .1,
    );
    canvas.drawCircle(
      Offset(size.width * .78, size.height * .12),
      size.width * .08,
      Paint()..color = const Color(0xffffa000),
    );
  }

  @override
  bool shouldRepaint(covariant _BombPainter oldDelegate) => false;
}

class _LedPainter extends CustomPainter {
  const _LedPainter(this.digits);

  final String digits;
  static const _segments = [
    0x3f,
    0x06,
    0x5b,
    0x4f,
    0x66,
    0x6d,
    0x7d,
    0x07,
    0x7f,
    0x6f,
  ];

  @override
  void paint(Canvas canvas, Size size) {
    const segments = [
      Rect.fromLTWH(3, 0, 11, 3),
      Rect.fromLTWH(14, 2, 3, 12),
      Rect.fromLTWH(14, 16, 3, 12),
      Rect.fromLTWH(3, 27, 11, 3),
      Rect.fromLTWH(0, 16, 3, 12),
      Rect.fromLTWH(0, 2, 3, 12),
      Rect.fromLTWH(3, 13.5, 11, 3),
    ];
    for (var digit = 0; digit < digits.length; digit++) {
      final mask = _segments[int.parse(digits[digit])];
      for (var segment = 0; segment < segments.length; segment++) {
        canvas.drawRRect(
          RRect.fromRectAndRadius(
            segments[segment].shift(Offset(digit * 21, 0)),
            const Radius.circular(1),
          ),
          Paint()
            ..color = mask & (1 << segment) != 0
                ? const Color(0xffff3040)
                : const Color(0xff3d1015),
        );
      }
    }
  }

  @override
  bool shouldRepaint(covariant _LedPainter oldDelegate) =>
      oldDelegate.digits != digits;
}

class _BevelButton extends StatelessWidget {
  const _BevelButton({
    super.key,
    required this.child,
    this.onTap,
    this.selected = false,
    this.width = 54,
  });

  final Widget child;
  final VoidCallback? onTap;
  final bool selected;
  final double width;

  @override
  Widget build(BuildContext context) => Semantics(
    button: true,
    selected: selected,
    enabled: onTap != null,
    child: GestureDetector(
      onTap: onTap,
      behavior: HitTestBehavior.opaque,
      child: Container(
        width: width,
        height: 44,
        padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 4),
        decoration: _bevel(
          selected ? const Color(0xffaaaaaa) : const Color(0xffc0c0c0),
          pressed: selected,
        ),
        child: Center(
          child: FittedBox(fit: BoxFit.scaleDown, child: child),
        ),
      ),
    ),
  );
}

class _PhoneBoard extends StatefulWidget {
  const _PhoneBoard({
    required this.controller,
    required this.flagMode,
    required this.onAction,
    required this.highlight,
    required this.highlightAnimation,
  });

  final GameController controller;
  final bool flagMode;
  final Future<void> Function(int x, int y, {required bool flag}) onAction;
  final GameHint? highlight;
  final Animation<double> highlightAnimation;

  @override
  State<_PhoneBoard> createState() => _PhoneBoardState();
}

class _PhoneBoardState extends State<_PhoneBoard> {
  GameSnapshot? _game;
  final List<ValueNotifier<GameCell>> _cells = [];

  @override
  void initState() {
    super.initState();
    widget.controller.gameState.addListener(_changed);
    _changed();
  }

  void _changed() {
    final game = widget.controller.game;
    final replace =
        game?.id != _game?.id || game?.cells.length != _cells.length;
    final statusChanged = game?.status != _game?.status;
    _game = game;
    if (replace) {
      for (final cell in _cells) {
        cell.dispose();
      }
      _cells.clear();
      if (game != null) _cells.addAll(game.cells.map(ValueNotifier.new));
    } else if (game != null) {
      for (var i = 0; i < _cells.length; i++) {
        if (_cells[i].value.state != game.cells[i].state ||
            _cells[i].value.value != game.cells[i].value) {
          _cells[i].value = game.cells[i];
        }
      }
    }
    if (mounted && (replace || statusChanged)) setState(() {});
  }

  @override
  void dispose() {
    widget.controller.gameState.removeListener(_changed);
    for (final cell in _cells) {
      cell.dispose();
    }
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final game = _game;
    if (game == null) return const CircularProgressIndicator();
    return LayoutBuilder(
      builder: (context, constraints) {
        final cellSize = math.max(
          0.0,
          math.min(
            (constraints.maxWidth - 8) / game.columns,
            (constraints.maxHeight - 8) / game.rows,
          ),
        );
        return Container(
          key: const ValueKey('mobile-board'),
          width: cellSize * game.columns + 8,
          height: cellSize * game.rows + 8,
          decoration: _bevel(const Color(0xffb0b0b0), pressed: true),
          padding: const EdgeInsets.all(1),
          child: GridView.builder(
            physics: const NeverScrollableScrollPhysics(),
            padding: EdgeInsets.zero,
            itemCount: _cells.length,
            gridDelegate: SliverGridDelegateWithFixedCrossAxisCount(
              crossAxisCount: game.columns,
            ),
            itemBuilder: (context, index) {
              final x = index % game.columns;
              final y = index ~/ game.columns;
              return ValueListenableBuilder<GameCell>(
                key: ValueKey('${game.id}-$index'),
                valueListenable: _cells[index],
                builder: (context, cell, _) => _PhoneCell(
                  cell: cell,
                  x: x,
                  y: y,
                  size: cellSize,
                  enabled: game.status == GameStatus.playing,
                  onTap: () => widget.onAction(x, y, flag: widget.flagMode),
                  onFlag: () => widget.onAction(x, y, flag: true),
                  highlight:
                      widget.highlight?.x == x && widget.highlight?.y == y
                      ? widget.highlightAnimation
                      : null,
                  hintKind: widget.highlight?.x == x && widget.highlight?.y == y
                      ? widget.highlight?.kind
                      : null,
                ),
              );
            },
          ),
        );
      },
    );
  }
}

class _PhoneCell extends StatelessWidget {
  const _PhoneCell({
    required this.cell,
    required this.x,
    required this.y,
    required this.size,
    required this.enabled,
    required this.onTap,
    required this.onFlag,
    required this.highlight,
    required this.hintKind,
  });

  final GameCell cell;
  final int x;
  final int y;
  final double size;
  final bool enabled;
  final VoidCallback onTap;
  final VoidCallback onFlag;
  final Animation<double>? highlight;
  final HintKind? hintKind;

  @override
  Widget build(BuildContext context) {
    final hidden =
        cell.state == CellState.unknown || cell.state == CellState.flag;
    final label = switch (cell.state) {
      CellState.unknown => '未挖开',
      CellState.flag => '旗',
      CellState.mine || CellState.exploded => '雷',
      CellState.revealed => cell.value == 0 ? '空格' : '${cell.value}',
    };
    final color = switch (cell.value) {
      1 => Colors.blue.shade800,
      2 => Colors.green.shade800,
      3 => Colors.red.shade800,
      _ => Colors.deepPurple.shade800,
    };
    return Semantics(
      button: true,
      enabled: enabled && hidden,
      label:
          '$label，第${y + 1}行第${x + 1}列'
          '${hintKind == HintKind.safe
              ? '，安全提示'
              : hintKind == HintKind.guess
              ? '，猜测提示'
              : ''}',
      child: RawGestureDetector(
        behavior: HitTestBehavior.opaque,
        gestures: enabled && hidden
            ? {
                TapGestureRecognizer:
                    GestureRecognizerFactoryWithHandlers<TapGestureRecognizer>(
                      TapGestureRecognizer.new,
                      (recognizer) => recognizer.onTap = onTap,
                    ),
                LongPressGestureRecognizer:
                    GestureRecognizerFactoryWithHandlers<
                      LongPressGestureRecognizer
                    >(
                      () => LongPressGestureRecognizer(
                        duration: const Duration(milliseconds: 400),
                      ),
                      (recognizer) => recognizer.onLongPress = onFlag,
                    ),
              }
            : const {},
        child: Container(
          alignment: Alignment.center,
          decoration: hidden
              ? _bevel(const Color(0xffc0c0c0))
              : BoxDecoration(
                  color: cell.state == CellState.exploded
                      ? const Color(0xffef7777)
                      : const Color(0xffbdbdbd),
                  border: Border.all(color: const Color(0xff888888), width: .5),
                ),
          child: Stack(
            fit: StackFit.expand,
            children: [
              Center(
                child: switch (cell.state) {
                  CellState.unknown => const SizedBox.shrink(),
                  CellState.flag => Icon(
                    Icons.flag,
                    color: Colors.red.shade800,
                    size: size * .65,
                  ),
                  CellState.mine || CellState.exploded => CustomPaint(
                    size: Size.square(size * .65),
                    painter: const _BombPainter(),
                  ),
                  CellState.revealed => Text(
                    cell.value == 0 ? '' : '${cell.value}',
                    style: TextStyle(
                      fontSize: size * .55,
                      fontWeight: FontWeight.bold,
                      color: color,
                      height: 1,
                    ),
                  ),
                },
              ),
              if (highlight != null)
                IgnorePointer(
                  child: AnimatedBuilder(
                    animation: highlight!,
                    builder: (context, _) {
                      final pulse =
                          (1 - math.cos(highlight!.value * math.pi * 4)) / 2;
                      return Container(
                        key: const ValueKey('solver-highlight'),
                        decoration: BoxDecoration(
                          color:
                              (hintKind == HintKind.safe
                                      ? Colors.green
                                      : Colors.yellow)
                                  .withValues(alpha: 0.15 + pulse * 0.35),
                          border: Border.all(
                            color:
                                (hintKind == HintKind.safe
                                        ? Colors.green.shade800
                                        : Colors.orange)
                                    .withValues(alpha: 0.45 + pulse * 0.55),
                            width: 3,
                          ),
                        ),
                      );
                    },
                  ),
                ),
            ],
          ),
        ),
      ),
    );
  }
}
