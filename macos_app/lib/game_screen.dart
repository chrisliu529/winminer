import 'dart:async';
import 'dart:io';
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
    _controller.gameState.addListener(_handleGameChanged);
    unawaited(_controller.initialize());
  }

  void _handleGameChanged() {
    final game = _controller.game;
    if (game != null) _scheduleWindowResize(game);
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
    _controller.gameState.removeListener(_handleGameChanged);
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
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
              padding: MediaQuery.sizeOf(context).width < 600
                  ? const EdgeInsets.fromLTRB(12, 12, 12, 16)
                  : const EdgeInsets.fromLTRB(24, 16, 24, 24),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  _GameToolbar(controller: _controller),
                  _ControllerErrorBanner(controller: _controller),
                  if (_windowResizeError != null) ...[
                    const SizedBox(height: 12),
                    _ErrorBanner(message: _windowResizeError!),
                  ],
                  const SizedBox(height: 16),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        _GameStatus(controller: _controller),
                        const SizedBox(height: 16),
                        Expanded(child: _Minefield(controller: _controller)),
                        const SizedBox(height: 10),
                        _GameHint(controller: _controller),
                      ],
                    ),
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }

  void _scheduleWindowResize(GameSnapshot game) {
    if (!Platform.isMacOS) return;
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
    return AnimatedBuilder(
      animation: controller,
      builder: (context, _) {
        GameLevel? selectedLevel;
        for (final level in controller.levels) {
          if (level.level == controller.selectedLevel) {
            selectedLevel = level;
            break;
          }
        }
        return LayoutBuilder(
          builder: (context, constraints) {
            return Wrap(
              spacing: 12,
              runSpacing: 12,
              crossAxisAlignment: WrapCrossAlignment.center,
              children: [
                SizedBox(
                  width: math.min(310.0, constraints.maxWidth).toDouble(),
                  child: InputDecorator(
                    decoration: const InputDecoration(
                      labelText: 'Level',
                      border: OutlineInputBorder(),
                      contentPadding: EdgeInsets.symmetric(
                        horizontal: 12,
                        vertical: 4,
                      ),
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
                        onChanged: controller.controlsBusy
                            ? null
                            : (level) {
                                if (level != null) {
                                  controller.selectLevel(level);
                                }
                              },
                      ),
                    ),
                  ),
                ),
                FilledButton.icon(
                  onPressed:
                      controller.controlsBusy || controller.levels.isEmpty
                      ? null
                      : controller.newGame,
                  icon: const Icon(Icons.refresh),
                  label: const Text('New game'),
                ),
                FilledButton.tonalIcon(
                  onPressed:
                      controller.controlsBusy ||
                          controller.autoPlaying ||
                          controller.game?.status != GameStatus.playing
                      ? null
                      : () => controller.botStep(),
                  icon: const Icon(Icons.auto_awesome),
                  label: const Text('Bot step'),
                ),
                OutlinedButton.icon(
                  onPressed: controller.autoPlaying
                      ? controller.toggleAutoPlay
                      : controller.controlsBusy ||
                            controller.game?.status != GameStatus.playing
                      ? null
                      : controller.toggleAutoPlay,
                  icon: Icon(
                    controller.autoPlaying ? Icons.pause : Icons.play_arrow,
                  ),
                  label: Text(controller.autoPlaying ? 'Pause' : 'Auto play'),
                ),
              ],
            );
          },
        );
      },
    );
  }
}

class _GameStatus extends StatelessWidget {
  const _GameStatus({required this.controller});

  final GameController controller;

  @override
  Widget build(BuildContext context) {
    return ValueListenableBuilder<GameSnapshot?>(
      valueListenable: controller.gameState,
      builder: (context, snapshot, _) {
        if (snapshot == null) return const SizedBox.shrink();
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
      },
    );
  }
}

class _ControllerErrorBanner extends StatelessWidget {
  const _ControllerErrorBanner({required this.controller});

  final GameController controller;

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: controller,
      builder: (context, _) {
        final error = controller.errorMessage;
        if (error == null) return const SizedBox.shrink();
        return Padding(
          padding: const EdgeInsets.only(top: 12),
          child: _ErrorBanner(message: error),
        );
      },
    );
  }
}

class _GameHint extends StatelessWidget {
  const _GameHint({required this.controller});

  final GameController controller;

  @override
  Widget build(BuildContext context) {
    final hint = Text(
      Platform.isAndroid
          ? 'Tap to reveal · Long-press to flag'
          : 'Click to reveal · Right-click or long-press to flag',
      textAlign: TextAlign.center,
      style: Theme.of(context).textTheme.bodySmall
          ?.copyWith(color: Theme.of(context).colorScheme.onSurfaceVariant),
    );
    return ValueListenableBuilder<GameSnapshot?>(
      valueListenable: controller.gameState,
      child: hint,
      builder: (context, game, child) =>
          game == null ? const SizedBox.shrink() : child!,
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
        Flexible(
          child: Text(
            label,
            style: Theme.of(context).textTheme.titleSmall
                ?.copyWith(color: color, fontWeight: FontWeight.w600),
          ),
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

class _Minefield extends StatefulWidget {
  const _Minefield({required this.controller});

  final GameController controller;
  static const double _cellSize = 34;
  static const double _spacing = 2;

  @override
  State<_Minefield> createState() => _MinefieldState();
}

class _MinefieldState extends State<_Minefield> {
  GameSnapshot? _snapshot;
  final List<ValueNotifier<GameCell>> _cells = [];
  bool _enabled = false;
  bool _loading = false;

  @override
  void initState() {
    super.initState();
    _snapshot = widget.controller.game;
    _loading = widget.controller.busy;
    _enabled = _canInteract(_snapshot);
    _replaceCells(_snapshot);
    widget.controller.gameState.addListener(_handleGameChanged);
    widget.controller.addListener(_handleControllerChanged);
  }

  @override
  void didUpdateWidget(covariant _Minefield oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.controller == widget.controller) return;
    oldWidget.controller.gameState.removeListener(_handleGameChanged);
    oldWidget.controller.removeListener(_handleControllerChanged);
    _disposeCells();
    _snapshot = widget.controller.game;
    _loading = widget.controller.busy;
    _enabled = _canInteract(_snapshot);
    _replaceCells(_snapshot);
    widget.controller.gameState.addListener(_handleGameChanged);
    widget.controller.addListener(_handleControllerChanged);
  }

  void _handleGameChanged() {
    final next = widget.controller.gameState.value;
    final previous = _snapshot;
    final boardChanged =
        next == null ||
        previous == null ||
        next.id != previous.id ||
        next.rows != previous.rows ||
        next.columns != previous.columns ||
        next.cells.length != _cells.length;

    _snapshot = next;
    if (boardChanged) {
      _replaceCells(next);
    } else {
      for (var index = 0; index < next.cells.length; index++) {
        final nextCell = next.cells[index];
        if (!_sameCell(_cells[index].value, nextCell)) {
          _cells[index].value = nextCell;
        }
      }
    }

    final enabled = _canInteract(next);
    if (mounted && (boardChanged || enabled != _enabled)) {
      setState(() => _enabled = enabled);
    } else {
      _enabled = enabled;
    }
  }

  void _handleControllerChanged() {
    final enabled = _canInteract(_snapshot);
    final loading = _snapshot == null && widget.controller.busy;
    if (enabled == _enabled && loading == _loading) return;
    setState(() {
      _enabled = enabled;
      _loading = loading;
    });
  }

  bool _canInteract(GameSnapshot? snapshot) =>
      !widget.controller.controlsBusy && snapshot?.status == GameStatus.playing;

  bool _sameCell(GameCell first, GameCell second) =>
      first.state == second.state && first.value == second.value;

  void _replaceCells(GameSnapshot? snapshot) {
    _disposeCells();
    if (snapshot != null) {
      _cells.addAll(snapshot.cells.map(ValueNotifier.new));
    }
  }

  void _disposeCells() {
    for (final cell in _cells) {
      cell.dispose();
    }
    _cells.clear();
  }

  @override
  void dispose() {
    widget.controller.gameState.removeListener(_handleGameChanged);
    widget.controller.removeListener(_handleControllerChanged);
    _disposeCells();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final snapshot = _snapshot;
    if (snapshot == null) {
      return Center(
        child: _loading
            ? const CircularProgressIndicator()
            : const Text('Unable to start a game.'),
      );
    }
    return LayoutBuilder(
      builder: (context, constraints) {
        final availableBoardWidth =
            constraints.maxWidth - 2 * _GameScreenState._boardFrameInset;
        final availableBoardHeight =
            constraints.maxHeight - 2 * _GameScreenState._boardFrameInset;
        final cellSize = Platform.isAndroid
            ? 32.0
            : math
                  .min(
                    _Minefield._cellSize,
                    math.min(
                      (availableBoardWidth -
                              (snapshot.columns - 1) * _Minefield._spacing) /
                          snapshot.columns,
                      (availableBoardHeight -
                              (snapshot.rows - 1) * _Minefield._spacing) /
                          snapshot.rows,
                    ),
                  )
                  .clamp(1.0, _Minefield._cellSize)
                  .toDouble();
        final boardWidth =
            snapshot.columns * cellSize +
            (snapshot.columns - 1) * _Minefield._spacing;
        final boardHeight =
            snapshot.rows * cellSize +
            (snapshot.rows - 1) * _Minefield._spacing;
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
                                crossAxisSpacing: _Minefield._spacing,
                                mainAxisSpacing: _Minefield._spacing,
                              ),
                          itemBuilder: (context, index) {
                            final x = index % snapshot.columns;
                            final y = index ~/ snapshot.columns;
                            return ValueListenableBuilder<GameCell>(
                              key: ValueKey('${snapshot.id}-$index'),
                              valueListenable: _cells[index],
                              builder: (context, cell, _) => _MineCell(
                                cell: cell,
                                x: x,
                                y: y,
                                cellSize: cellSize,
                                enabled: _enabled,
                                onClick: () => widget.controller.click(x, y),
                                onFlag: () =>
                                    widget.controller.toggleFlag(x, y),
                              ),
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
