import 'dart:async';
import 'dart:math' as math;

import 'package:flutter/services.dart';
import 'package:material_ui/material_ui.dart';

import 'game_api.dart';
import 'game_controller.dart';
import 'game_models.dart';

class GameScreen extends StatefulWidget {
  const GameScreen({super.key, required this.api});

  final GameApi api;

  @override
  State<GameScreen> createState() => _GameScreenState();
}

class _GameScreenState extends State<GameScreen> with WidgetsBindingObserver {
  static const _windowChannel = MethodChannel('winminer/window');
  static const _windowMinWidth = 560.0;
  static const _toolbarSingleRowWidth = 800.0;
  static const _statusSingleRowWidth = 600.0;
  static const _boardFrameInset = 12.0;
  static const _baseWindowChromeHeight =
      kToolbarHeight +
      16 +
      24 +
      48 +
      16 +
      56 +
      16 +
      17 +
      10 +
      2 * _boardFrameInset +
      2;

  late final GameController _controller;
  int? _windowRows;
  int? _windowColumns;
  String? _windowResizeError;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _controller = GameController(widget.api);
    unawaited(_controller.initialize());
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.inactive ||
        state == AppLifecycleState.hidden ||
        state == AppLifecycleState.paused) {
      _controller.pauseAutoPlay();
    }
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: _controller,
      builder: (context, _) {
        final game = _controller.game;
        if (game != null) _scheduleWindowResize(game);
        return Scaffold(
          appBar: AppBar(
            title: const Text('Winminer'),
            backgroundColor: Theme.of(context).colorScheme.surface,
          ),
          body: SafeArea(
            child: Center(
              child: ConstrainedBox(
                constraints: const BoxConstraints(maxWidth: 1120),
                child: Padding(
                  padding: const EdgeInsets.fromLTRB(24, 16, 24, 24),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      _GameToolbar(controller: _controller),
                      if (_controller.errorMessage != null) ...[
                        const SizedBox(height: 12),
                        _ErrorBanner(message: _controller.errorMessage!),
                      ],
                      if (_windowResizeError != null) ...[
                        const SizedBox(height: 12),
                        _ErrorBanner(message: _windowResizeError!),
                      ],
                      if (game == null) ...[
                        const Spacer(),
                        if (_controller.busy)
                          const Center(child: CircularProgressIndicator())
                        else
                          const Center(child: Text('Unable to start a game.')),
                        const Spacer(),
                      ] else ...[
                        const SizedBox(height: 16),
                        _GameStatus(snapshot: game),
                        const SizedBox(height: 16),
                        Expanded(
                          child: _Minefield(
                            snapshot: game,
                            enabled:
                                !_controller.busy &&
                                game.status == GameStatus.playing,
                            onClick: _controller.click,
                            onFlag: _controller.toggleFlag,
                          ),
                        ),
                        const SizedBox(height: 10),
                        Text(
                          'Click to reveal · Right-click or long-press to flag',
                          textAlign: TextAlign.center,
                          style: Theme.of(context).textTheme.bodySmall
                              ?.copyWith(
                                color: Theme.of(context)
                                    .colorScheme
                                    .onSurfaceVariant,
                              ),
                        ),
                      ],
                    ],
                  ),
                ),
              ),
            ),
          ),
        );
      },
    );
  }

  void _scheduleWindowResize(GameSnapshot game) {
    if (_windowRows == game.rows && _windowColumns == game.columns) return;
    _windowRows = game.rows;
    _windowColumns = game.columns;

    final boardWidth =
        game.columns * _Minefield._cellSize +
        (game.columns - 1) * _Minefield._spacing;
    final boardHeight =
        game.rows * _Minefield._cellSize +
        (game.rows - 1) * _Minefield._spacing;
    final width = math
        .max(_windowMinWidth, boardWidth + 2 * 24 + 2 * _boardFrameInset + 2)
        .toDouble();
    final height =
        boardHeight +
        _baseWindowChromeHeight +
        (width < _toolbarSingleRowWidth ? 60 : 0) +
        (width < _statusSingleRowWidth ? 30 : 0);

    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) unawaited(_resizeWindow(width, height));
    });
  }

  Future<void> _resizeWindow(double width, double height) async {
    try {
      await _windowChannel.invokeMethod<void>('resizeToFitBoard', {
        'width': width,
        'height': height,
      });
      if (mounted && _windowResizeError != null) {
        setState(() => _windowResizeError = null);
      }
    } on PlatformException catch (error) {
      _reportWindowResizeError(error);
    } on MissingPluginException catch (error) {
      _reportWindowResizeError(error);
    }
  }

  void _reportWindowResizeError(Object error) {
    if (!mounted) return;
    setState(() => _windowResizeError = 'Unable to resize window: $error');
  }
}

class _GameToolbar extends StatelessWidget {
  const _GameToolbar({required this.controller});

  final GameController controller;

  @override
  Widget build(BuildContext context) {
    GameLevel? selectedLevel;
    for (final level in controller.levels) {
      if (level.level == controller.selectedLevel) {
        selectedLevel = level;
        break;
      }
    }
    return Wrap(
      spacing: 12,
      runSpacing: 12,
      crossAxisAlignment: WrapCrossAlignment.center,
      children: [
        SizedBox(
          width: 310,
          child: InputDecorator(
            decoration: const InputDecoration(
              labelText: 'Level',
              border: OutlineInputBorder(),
              contentPadding: EdgeInsets.symmetric(horizontal: 12, vertical: 4),
            ),
            child: DropdownButtonHideUnderline(
              child: DropdownButton<int>(
                value: selectedLevel?.level,
                isExpanded: true,
                items: controller.levels.map((level) {
                  return DropdownMenuItem(
                    value: level.level,
                    child: Text(
                      'Level ${level.level} · ${level.rows}×${level.columns} · ${level.mines} mines',
                      overflow: TextOverflow.ellipsis,
                    ),
                  );
                }).toList(),
                onChanged: controller.busy
                    ? null
                    : (level) {
                        if (level != null) controller.selectLevel(level);
                      },
              ),
            ),
          ),
        ),
        FilledButton.icon(
          onPressed: controller.busy || controller.levels.isEmpty
              ? null
              : controller.newGame,
          icon: const Icon(Icons.refresh),
          label: const Text('New game'),
        ),
        FilledButton.tonalIcon(
          onPressed:
              controller.busy || controller.game?.status != GameStatus.playing
              ? null
              : controller.botStep,
          icon: const Icon(Icons.auto_awesome),
          label: const Text('Bot step'),
        ),
        OutlinedButton.icon(
          onPressed: controller.autoPlaying
              ? controller.toggleAutoPlay
              : controller.busy || controller.game?.status != GameStatus.playing
              ? null
              : controller.toggleAutoPlay,
          icon: Icon(controller.autoPlaying ? Icons.pause : Icons.play_arrow),
          label: Text(controller.autoPlaying ? 'Pause' : 'Auto play'),
        ),
      ],
    );
  }
}

class _GameStatus extends StatelessWidget {
  const _GameStatus({required this.snapshot});

  final GameSnapshot snapshot;

  @override
  Widget build(BuildContext context) {
    final (label, icon, color) = switch (snapshot.status) {
      GameStatus.playing => (
        'Playing',
        Icons.sports_esports_outlined,
        Theme.of(context).colorScheme.primary,
      ),
      GameStatus.won => (
        'You won!',
        Icons.emoji_events_outlined,
        Colors.green.shade700,
      ),
      GameStatus.lost => (
        'Game over',
        Icons.sentiment_dissatisfied_outlined,
        Theme.of(context).colorScheme.error,
      ),
    };
    return Card(
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 14),
        child: Wrap(
          spacing: 28,
          runSpacing: 10,
          crossAxisAlignment: WrapCrossAlignment.center,
          children: [
            _StatusItem(icon: icon, label: label, color: color),
            _StatusItem(
              icon: Icons.flag_outlined,
              label: '${snapshot.remainingMines} mines remaining',
            ),
            _StatusItem(
              icon: Icons.grid_on_outlined,
              label:
                  '${snapshot.rows}×${snapshot.columns} · ${snapshot.mines} mines',
            ),
          ],
        ),
      ),
    );
  }
}

class _StatusItem extends StatelessWidget {
  const _StatusItem({required this.icon, required this.label, this.color});

  final IconData icon;
  final String label;
  final Color? color;

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(icon, size: 19, color: color),
        const SizedBox(width: 8),
        Text(
          label,
          style: Theme.of(context).textTheme.titleSmall
              ?.copyWith(color: color, fontWeight: FontWeight.w600),
        ),
      ],
    );
  }
}

class _ErrorBanner extends StatelessWidget {
  const _ErrorBanner({required this.message});

  final String message;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: Theme.of(context).colorScheme.errorContainer,
      borderRadius: BorderRadius.circular(8),
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Row(
          children: [
            Icon(
              Icons.error_outline,
              color: Theme.of(context).colorScheme.error,
            ),
            const SizedBox(width: 10),
            Expanded(child: Text(message)),
          ],
        ),
      ),
    );
  }
}

class _Minefield extends StatelessWidget {
  const _Minefield({
    required this.snapshot,
    required this.enabled,
    required this.onClick,
    required this.onFlag,
  });

  static const double _cellSize = 34;
  static const double _spacing = 2;

  final GameSnapshot snapshot;
  final bool enabled;
  final Future<void> Function(int x, int y) onClick;
  final Future<void> Function(int x, int y) onFlag;

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) {
        final availableBoardWidth =
            constraints.maxWidth - 2 * _GameScreenState._boardFrameInset;
        final availableBoardHeight =
            constraints.maxHeight - 2 * _GameScreenState._boardFrameInset;
        final cellSize = math
            .min(
              _cellSize,
              math.min(
                (availableBoardWidth - (snapshot.columns - 1) * _spacing) /
                    snapshot.columns,
                (availableBoardHeight - (snapshot.rows - 1) * _spacing) /
                    snapshot.rows,
              ),
            )
            .clamp(1.0, _cellSize)
            .toDouble();
        final boardWidth =
            snapshot.columns * cellSize + (snapshot.columns - 1) * _spacing;
        final boardHeight =
            snapshot.rows * cellSize + (snapshot.rows - 1) * _spacing;
        return DecoratedBox(
          decoration: BoxDecoration(
            color: const Color(0xffb7c1ca),
            border: Border.all(color: const Color(0xff9ca8b3)),
            borderRadius: BorderRadius.circular(12),
          ),
          child: ClipRRect(
            borderRadius: BorderRadius.circular(11),
            child: Padding(
              padding: EdgeInsets.all(_GameScreenState._boardFrameInset),
              child: SingleChildScrollView(
                child: SingleChildScrollView(
                  scrollDirection: Axis.horizontal,
                  child: ConstrainedBox(
                    constraints: BoxConstraints(
                      minWidth:
                          constraints.maxWidth -
                          2 * _GameScreenState._boardFrameInset,
                    ),
                    child: Center(
                      child: SizedBox(
                        width: boardWidth,
                        height: boardHeight,
                        child: GridView.builder(
                          padding: EdgeInsets.zero,
                          physics: const NeverScrollableScrollPhysics(),
                          itemCount: snapshot.cells.length,
                          gridDelegate:
                              SliverGridDelegateWithFixedCrossAxisCount(
                                crossAxisCount: snapshot.columns,
                                crossAxisSpacing: _spacing,
                                mainAxisSpacing: _spacing,
                              ),
                          itemBuilder: (context, index) {
                            final x = index % snapshot.columns;
                            final y = index ~/ snapshot.columns;
                            return _MineCell(
                              key: ValueKey('${snapshot.id}-$index'),
                              cell: snapshot.cells[index],
                              x: x,
                              y: y,
                              cellSize: cellSize,
                              enabled: enabled,
                              onClick: () => onClick(x, y),
                              onFlag: () => onFlag(x, y),
                            );
                          },
                        ),
                      ),
                    ),
                  ),
                ),
              ),
            ),
          ),
        );
      },
    );
  }
}

class _MineCell extends StatelessWidget {
  const _MineCell({
    super.key,
    required this.cell,
    required this.x,
    required this.y,
    required this.cellSize,
    required this.enabled,
    required this.onClick,
    required this.onFlag,
  });

  final GameCell cell;
  final int x;
  final int y;
  final double cellSize;
  final bool enabled;
  final Future<void> Function() onClick;
  final Future<void> Function() onFlag;

  @override
  Widget build(BuildContext context) {
    final revealed = cell.state == CellState.revealed;
    final interactive =
        enabled && !revealed && cell.state != CellState.exploded;
    final background = switch (cell.state) {
      CellState.unknown || CellState.flag => const Color(0xffcad3db),
      CellState.revealed => const Color(0xfff7f9fa),
      CellState.exploded => const Color(0xfff4a3a0),
    };
    final border = revealed ? const Color(0xffd3dae0) : const Color(0xff9ca8b3);
    final label = switch (cell.state) {
      CellState.unknown => 'Hidden at ${x + 1}, ${y + 1}',
      CellState.flag => 'Flag at ${x + 1}, ${y + 1}',
      CellState.exploded => 'Mine at ${x + 1}, ${y + 1}',
      CellState.revealed =>
        cell.value == 0
            ? 'Empty at ${x + 1}, ${y + 1}'
            : '${cell.value} at ${x + 1}, ${y + 1}',
    };
    final child = switch (cell.state) {
      CellState.unknown => const SizedBox.expand(),
      CellState.flag => Icon(
        Icons.flag,
        size: math.min(18, cellSize * 0.55).toDouble(),
        color: Color(0xffa12622),
      ),
      CellState.exploded => Icon(
        Icons.close,
        size: math.min(20, cellSize * 0.6).toDouble(),
        color: Color(0xff7e1814),
      ),
      CellState.revealed =>
        cell.value == 0
            ? const SizedBox.expand()
            : Text(
                '${cell.value}',
                style: TextStyle(
                  color: _numberColor(cell.value!),
                  fontWeight: FontWeight.w800,
                  fontSize: math.min(17, cellSize * 0.5).toDouble(),
                ),
              ),
    };
    return Semantics(
      label: label,
      button: interactive,
      enabled: interactive,
      child: Tooltip(
        message: label,
        waitDuration: const Duration(milliseconds: 700),
        child: MouseRegion(
          cursor: interactive ? SystemMouseCursors.click : MouseCursor.defer,
          child: GestureDetector(
            onTap: interactive && cell.state == CellState.unknown
                ? onClick
                : null,
            onLongPress: interactive ? onFlag : null,
            onSecondaryTap: interactive ? onFlag : null,
            child: Container(
              alignment: Alignment.center,
              decoration: BoxDecoration(
                color: background,
                border: Border.all(color: border),
              ),
              child: child,
            ),
          ),
        ),
      ),
    );
  }

  Color _numberColor(int value) => switch (value) {
    1 => const Color(0xff1762b3),
    2 => const Color(0xff218044),
    3 => const Color(0xffc3332c),
    4 => const Color(0xff573c9a),
    _ => const Color(0xff243746),
  };
}
