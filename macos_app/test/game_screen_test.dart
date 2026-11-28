import 'dart:math' as math;
import 'dart:typed_data';

import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:winminer_app/game_api.dart';
import 'package:winminer_app/game_controller.dart';
import 'package:winminer_app/game_models.dart';
import 'package:winminer_app/game_screen.dart';
import 'package:winminer_app/mobile_game_screen.dart';

const _windowChannel = MethodChannel('winminer/window');

void main() {
  testWidgets('action sounds are PCM WAVs at matching levels', (tester) async {
    final levels = <double>[];
    for (final sound in ActionSound.values) {
      final data = await rootBundle.load(
        'assets/audio/${sound.name}_effect.wav',
      );
      expect(
        String.fromCharCodes(data.buffer.asUint8List(data.offsetInBytes, 4)),
        'RIFF',
      );
      expect(
        String.fromCharCodes(
          data.buffer.asUint8List(data.offsetInBytes + 8, 8),
        ),
        'WAVEfmt ',
      );
      expect(data.getUint16(20, Endian.little), 1);
      expect(data.getUint16(22, Endian.little), 1);
      expect(data.getUint32(24, Endian.little), 44100);
      expect(data.getUint16(34, Endian.little), 16);
      expect(
        String.fromCharCodes(
          data.buffer.asUint8List(data.offsetInBytes + 36, 4),
        ),
        'data',
      );
      final length = data.getUint32(40, Endian.little);
      expect(length, greaterThan(0));
      expect(length + 44, data.lengthInBytes);
      var sumSquares = 0.0;
      var peak = 0.0;
      for (var offset = 44; offset < data.lengthInBytes; offset += 2) {
        final sample = data.getInt16(offset, Endian.little) / 32768;
        peak = math.max(peak, sample.abs());
        sumSquares += sample * sample;
      }
      expect(peak, lessThan(0.99));
      levels.add(math.sqrt(sumSquares / (length / 2)));
    }
    final reference = levels[ActionSound.flag.index];
    for (final level in levels) {
      expect(
        (20 * math.log(level / reference) / math.ln10).abs(),
        lessThan(0.1),
      );
    }
  });

  test(
    'automatic steps publish game changes without control notifications',
    () async {
      final controller = GameController(_FakeGameApi());
      await controller.initialize();
      var controlUpdates = 0;
      var gameUpdates = 0;
      controller.addListener(() => controlUpdates++);
      controller.gameState.addListener(() => gameUpdates++);

      controller.toggleAutoPlay();
      await Future<void>.delayed(Duration.zero);

      expect(gameUpdates, 1);
      expect(controlUpdates, 1);
      controller.pauseAutoPlay();
      expect(controlUpdates, 2);
      controller.dispose();
    },
  );

  testWidgets('renders the game and sends board actions to the API', (
    tester,
  ) async {
    _mockWindowResizing();
    addTearDown(_clearWindowResizingMock);
    final api = _FakeGameApi();
    await tester.pumpWidget(MaterialApp(home: GameScreen(api: api)));
    await tester.pumpAndSettle();

    expect(find.text('Winminer'), findsOneWidget);
    expect(find.text('1 mines remaining'), findsOneWidget);
    expect(find.byKey(const ValueKey('game-1-0')), findsOneWidget);
    final cell = tester.widget<Container>(
      find
          .descendant(
            of: find.byKey(const ValueKey('game-1-0')),
            matching: find.byType(Container),
          )
          .first,
    );
    expect((cell.decoration! as BoxDecoration).borderRadius, isNull);

    await tester.longPress(find.byKey(const ValueKey('game-1-0')));
    await tester.pumpAndSettle();
    expect(api.lastFlaggedCell, (0, 0));
    expect(find.text('0 mines remaining'), findsOneWidget);

    await tester.tap(find.text('Bot step'));
    await tester.pumpAndSettle();
    expect(api.botSteps, 1);
    expect(find.text('1'), findsOneWidget);
  });

  testWidgets('revealing a cell does not rebuild the status labels', (
    tester,
  ) async {
    _mockWindowResizing();
    addTearDown(_clearWindowResizingMock);
    final api = _FakeGameApi();
    await tester.pumpWidget(MaterialApp(home: GameScreen(api: api)));
    await tester.pumpAndSettle();
    final statusLabel = tester.widget<Text>(find.text('Playing'));

    await tester.tap(find.byKey(const ValueKey('game-1-1')));
    await tester.pumpAndSettle();

    expect(
      identical(statusLabel, tester.widget<Text>(find.text('Playing'))),
      isTrue,
    );
  });

  testWidgets('keeps the minefield size stable through flags and game end', (
    tester,
  ) async {
    _mockWindowResizing();
    addTearDown(_clearWindowResizingMock);
    tester.view.physicalSize = const Size(320, 640);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    final api = _FakeGameApi(
      levels: const [GameLevel(level: 1, rows: 16, columns: 16, mines: 99)],
      clickStatus: GameStatus.lost,
    );
    await tester.pumpWidget(MaterialApp(home: GameScreen(api: api)));
    await tester.pumpAndSettle();
    final frame = find.byKey(const ValueKey('minefield-frame'));
    final board = find.byKey(const ValueKey('minefield-board'));
    final initialFrameSize = tester.getSize(frame);
    final initialBoardSize = tester.getSize(board);

    await tester.longPress(find.byKey(const ValueKey('game-1-0')));
    await tester.pumpAndSettle();
    expect(tester.getSize(frame), initialFrameSize);
    expect(tester.getSize(board), initialBoardSize);

    await tester.tap(find.byKey(const ValueKey('game-1-1')));
    await tester.pumpAndSettle();
    expect(find.text('Game over'), findsOneWidget);
    expect(tester.getSize(frame), initialFrameSize);
    expect(tester.getSize(board), initialBoardSize);
  });

  testWidgets('shows a bomb icon for an exploded cell', (tester) async {
    _mockWindowResizing();
    addTearDown(_clearWindowResizingMock);
    final api = _FakeGameApi(
      clickResult: const GameCell(state: CellState.exploded),
    );
    await tester.pumpWidget(MaterialApp(home: GameScreen(api: api)));
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const ValueKey('game-1-0')));
    await tester.pumpAndSettle();

    expect(find.text('💣'), findsOneWidget);
  });

  testWidgets('shows fireworks when the game is won', (tester) async {
    _mockWindowResizing();
    addTearDown(_clearWindowResizingMock);
    final api = _FakeGameApi(clickStatus: GameStatus.won);
    await tester.pumpWidget(MaterialApp(home: GameScreen(api: api)));
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const ValueKey('game-1-0')));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 200));

    expect(find.text('You won!'), findsOneWidget);
    expect(find.byKey(const ValueKey('victory-fireworks')), findsOneWidget);
    await tester.pump(const Duration(seconds: 3));
    expect(find.byKey(const ValueKey('victory-fireworks')), findsOneWidget);
    await tester.pump(const Duration(milliseconds: 1400));
    expect(find.byKey(const ValueKey('victory-fireworks')), findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets('resizes the window when the active board size changes', (
    tester,
  ) async {
    final resizeCalls = _mockWindowResizing();
    addTearDown(_clearWindowResizingMock);
    final api = _FakeGameApi(
      levels: const [
        GameLevel(level: 1, rows: 2, columns: 2, mines: 1),
        GameLevel(level: 2, rows: 16, columns: 30, mines: 99),
      ],
    );
    await tester.pumpWidget(MaterialApp(home: GameScreen(api: api)));
    await tester.pumpAndSettle();
    expect(resizeCalls, hasLength(1));
    final firstSize = Map<String, double>.from(
      resizeCalls.first.arguments as Map,
    );

    await tester.tap(find.byType(DropdownButton<int>));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Level 2 · 16×30 · 99 mines'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('New game'));
    await tester.pumpAndSettle();

    expect(resizeCalls, hasLength(2));
    final secondSize = Map<String, double>.from(
      resizeCalls.last.arguments as Map,
    );
    expect(secondSize['width'], greaterThan(firstSize['width']!));
    expect(secondSize['height'], greaterThan(firstSize['height']!));
  });

  testWidgets('lays out controls on a narrow phone-sized viewport', (
    tester,
  ) async {
    _mockWindowResizing();
    addTearDown(_clearWindowResizingMock);
    tester.view.physicalSize = const Size(320, 640);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(MaterialApp(home: GameScreen(api: _FakeGameApi())));
    await tester.pumpAndSettle();

    final exception = tester.takeException();
    expect(exception, isNull);
    expect(find.text('New game'), findsOneWidget);
    expect(find.byKey(const ValueKey('game-1-0')), findsOneWidget);
  });

  testWidgets('phone controls flag at 400ms and switch tap mode', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(320, 640);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    final api = _FakeGameApi();
    await tester.pumpWidget(
      MaterialApp(
        home: GameScreen(api: api, mobile: true, onActionSound: (_) {}),
      ),
    );
    await tester.pumpAndSettle();
    final gesture = await tester.startGesture(
      tester.getCenter(find.byKey(const ValueKey('game-1-0'))),
    );
    await tester.pump(const Duration(milliseconds: 399));
    expect(api.lastFlaggedCell, isNull);
    await tester.pump(const Duration(milliseconds: 2));
    await tester.pump();
    expect(api.lastFlaggedCell, (0, 0));
    await gesture.up();
    await tester.pump();
    expect(find.byIcon(Icons.flag), findsOneWidget);

    await tester.tap(find.byKey(const ValueKey('flag-mode')));
    await tester.pump();
    await tester.tap(find.byKey(const ValueKey('game-1-1')));
    await tester.pump();
    expect(api.lastFlaggedCell, (1, 0));
    expect(find.byIcon(Icons.flag), findsNWidgets(2));
    expect(tester.takeException(), isNull);
  });

  testWidgets('phone plays action sounds only for successful manual moves', (
    tester,
  ) async {
    final api = _FakeGameApi();
    final controller = GameController(api);
    await controller.initialize();
    final sounds = <ActionSound>[];
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: MobileGameScreen(
            controller: controller,
            onActionSound: sounds.add,
          ),
        ),
      ),
    );
    await tester.tap(find.byKey(const ValueKey('game-1-0')));
    await tester.pump();
    expect(sounds, [ActionSound.reveal]);
    await tester.longPress(find.byKey(const ValueKey('game-1-1')));
    await tester.pump();
    expect(sounds, [ActionSound.reveal, ActionSound.flag]);
    await tester.tap(find.byKey(const ValueKey('flag-mode')));
    await tester.pump();
    await tester.tap(find.byKey(const ValueKey('game-1-1')));
    await tester.pump();
    expect(sounds, [ActionSound.reveal, ActionSound.flag, ActionSound.flag]);
    api.failFlag = true;
    await tester.tap(find.byKey(const ValueKey('game-1-2')));
    await tester.pump();
    expect(sounds, [ActionSound.reveal, ActionSound.flag, ActionSound.flag]);
    await tester.tap(find.byKey(const ValueKey('flag-mode')));
    await tester.pump();
    api.failClick = true;
    await tester.tap(find.byKey(const ValueKey('game-1-2')));
    await tester.pump();
    expect(sounds, [ActionSound.reveal, ActionSound.flag, ActionSound.flag]);
    await tester.tap(find.byKey(const ValueKey('game-1-0')));
    await tester.pump();
    expect(sounds, [ActionSound.reveal, ActionSound.flag, ActionSound.flag]);
    api.failClick = false;
    api.clickStatus = GameStatus.lost;
    await tester.tap(find.byKey(const ValueKey('game-1-2')));
    await tester.pump();
    expect(sounds, [
      ActionSound.reveal,
      ActionSound.flag,
      ActionSound.flag,
      ActionSound.mine,
    ]);
    await tester.pumpWidget(const SizedBox.shrink());
    controller.dispose();
  });

  testWidgets('phone solver appears after inactivity and resets on new game', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(320, 640);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    final api = _FakeGameApi();
    await tester.pumpWidget(
      MaterialApp(
        home: GameScreen(api: api, mobile: true, onActionSound: (_) {}),
      ),
    );
    await tester.pumpAndSettle();
    await tester.pump(const Duration(seconds: 6));
    expect(find.byKey(const ValueKey('bot-step')), findsNothing);
    final restartButton = tester.widget(find.byKey(const ValueKey('restart')));
    await tester.tap(find.byKey(const ValueKey('game-1-0')));
    await tester.pump();
    expect(
      identical(
        restartButton,
        tester.widget(find.byKey(const ValueKey('restart'))),
      ),
      isTrue,
    );
    await tester.pump(const Duration(seconds: 4));
    expect(find.byKey(const ValueKey('bot-step')), findsNothing);
    await tester.longPress(find.byKey(const ValueKey('game-1-2')));
    await tester.pump();
    await tester.pump(const Duration(seconds: 4));
    expect(find.byKey(const ValueKey('bot-step')), findsNothing);
    await tester.pump(const Duration(seconds: 1));
    expect(find.byKey(const ValueKey('bot-step')), findsOneWidget);
    expect(
      tester.getTopLeft(find.byKey(const ValueKey('flag-mode'))).dx -
          tester.getTopRight(find.byKey(const ValueKey('bot-step'))).dx,
      4,
    );
    await tester.tap(find.byKey(const ValueKey('bot-step')));
    await tester.pump();
    expect(api.hints, 1);
    expect(api.botSteps, 0);
    expect(
      find.descendant(
        of: find.byKey(const ValueKey('game-1-1')),
        matching: find.byKey(const ValueKey('solver-highlight')),
      ),
      findsOneWidget,
    );
    await tester.tap(find.byKey(const ValueKey('restart')));
    await tester.pump();
    expect(find.byKey(const ValueKey('bot-step')), findsNothing);
    expect(find.byKey(const ValueKey('solver-highlight')), findsNothing);
    expect(find.byKey(const ValueKey('game-2-0')), findsOneWidget);
    await tester.pumpWidget(const SizedBox.shrink());
  });

  for (final kind in HintKind.values) {
    testWidgets('phone hint highlights $kind without opening a cell', (
      tester,
    ) async {
      final api = _FakeGameApi(hintKind: kind);
      final sounds = <ActionSound>[];
      await tester.pumpWidget(
        MaterialApp(
          home: GameScreen(api: api, mobile: true, onActionSound: sounds.add),
        ),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('game-1-0')));
      await tester.pump();
      await tester.pump(const Duration(seconds: 5));
      await tester.tap(find.byKey(const ValueKey('bot-step')));
      await tester.pump();
      expect(sounds, [ActionSound.reveal]);
      expect(api.botSteps, 0);
      expect(api.hints, 1);
      expect(api._games['game-1']!.status, GameStatus.playing);
      expect(api._games['game-1']!.cells[1].state, CellState.unknown);
      expect(
        find.descendant(
          of: find.byKey(const ValueKey('game-1-1')),
          matching: find.byKey(const ValueKey('solver-highlight')),
        ),
        findsOneWidget,
      );
      expect(
        find.descendant(
          of: find.byKey(const ValueKey('game-1-2')),
          matching: find.byKey(const ValueKey('solver-highlight')),
        ),
        findsNothing,
      );
      final initialColor =
          (tester
                      .widget<Container>(
                        find.byKey(const ValueKey('solver-highlight')),
                      )
                      .decoration!
                  as BoxDecoration)
              .color;
      expect(
        initialColor,
        (kind == HintKind.safe ? Colors.green : Colors.yellow).withValues(
          alpha: 0.15,
        ),
      );
      await tester.pump(const Duration(milliseconds: 250));
      final pulseColor =
          (tester
                      .widget<Container>(
                        find.byKey(const ValueKey('solver-highlight')),
                      )
                      .decoration!
                  as BoxDecoration)
              .color;
      expect(pulseColor, isNot(initialColor));
      await tester.pump(const Duration(milliseconds: 751));
      expect(find.byKey(const ValueKey('solver-highlight')), findsNothing);
      await tester.pumpWidget(const SizedBox.shrink());
    });
  }

  testWidgets('phone reports invalid hints without changing the board', (
    tester,
  ) async {
    final controller = GameController(_FakeGameApi(hintX: 9));
    await controller.initialize();
    final game = controller.game;
    await controller.hint();
    expect(controller.errorMessage, contains('invalid hint tile'));
    expect(identical(controller.game, game), isTrue);
    controller.dispose();
  });

  testWidgets('phone timer starts on reveal, survives background and stops', (
    tester,
  ) async {
    final api = _FakeGameApi();
    final controller = GameController(api);
    await controller.initialize();
    var now = DateTime(2026, 1, 1);
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: MobileGameScreen(
            controller: controller,
            onActionSound: (_) {},
            now: () => now,
          ),
        ),
      ),
    );
    expect(find.bySemanticsLabel('000'), findsOneWidget);
    await tester.longPress(find.byKey(const ValueKey('game-1-0')));
    await tester.pump();
    now = now.add(const Duration(seconds: 10));
    await tester.pump(const Duration(seconds: 1));
    expect(find.bySemanticsLabel('000'), findsNWidgets(2));
    await tester.tap(find.byKey(const ValueKey('game-1-1')));
    await tester.pump();
    now = now.add(const Duration(seconds: 13));
    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
    await tester.pump();
    expect(find.bySemanticsLabel('013'), findsOneWidget);
    expect(find.byKey(const ValueKey('bot-step')), findsOneWidget);
    api.clickStatus = GameStatus.lost;
    await tester.tap(find.byKey(const ValueKey('game-1-2')));
    await tester.pump();
    now = now.add(const Duration(seconds: 20));
    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
    await tester.pump();
    expect(find.bySemanticsLabel('013'), findsOneWidget);
    await tester.pumpWidget(const SizedBox.shrink());
    controller.dispose();
  });

  testWidgets('phone difficulty changes board size and timer caps at 999', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(320, 640);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    final controller = GameController(
      _FakeGameApi(
        levels: const [
          GameLevel(level: 1, rows: 8, columns: 8, mines: 10),
          GameLevel(level: 2, rows: 10, columns: 10, mines: 20),
          GameLevel(level: 3, rows: 14, columns: 10, mines: 36),
        ],
      ),
    );
    await controller.initialize();
    var now = DateTime(2026, 1, 1);
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: MobileGameScreen(
            controller: controller,
            onActionSound: (_) {},
            now: () => now,
          ),
        ),
      ),
    );
    expect(find.byKey(const ValueKey('game-1-63')), findsOneWidget);
    await tester.tap(find.byKey(const ValueKey('level-2')));
    await tester.pump();
    expect(find.byKey(const ValueKey('game-2-99')), findsOneWidget);
    final mediumCell = tester.getSize(find.byKey(const ValueKey('game-2-0')));
    expect(
      tester.getSize(find.byKey(const ValueKey('mobile-board'))).width,
      greaterThan(250),
    );
    await tester.tap(find.byKey(const ValueKey('level-3')));
    await tester.pump();
    expect(find.byKey(const ValueKey('game-3-139')), findsOneWidget);
    expect(find.bySemanticsLabel('036'), findsOneWidget);
    expect(tester.getSize(find.byKey(const ValueKey('game-3-0'))), mediumCell);
    await tester.tap(find.byKey(const ValueKey('game-3-139')));
    await tester.pump();
    now = now.add(const Duration(seconds: 1002));
    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
    await tester.pump();
    expect(find.bySemanticsLabel('999'), findsOneWidget);
    expect(tester.takeException(), isNull);
    await tester.pumpWidget(const SizedBox.shrink());
    controller.dispose();
  });

  testWidgets('advanced phone board fits short and landscape screens', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(320, 480);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    final controller = GameController(
      _FakeGameApi(
        levels: const [
          GameLevel(level: 1, rows: 8, columns: 8, mines: 10),
          GameLevel(level: 2, rows: 10, columns: 10, mines: 20),
          GameLevel(level: 3, rows: 14, columns: 10, mines: 36),
        ],
      ),
    );
    await controller.initialize();
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: MobileGameScreen(controller: controller, onActionSound: (_) {}),
        ),
      ),
    );
    await tester.tap(find.byKey(const ValueKey('level-3')));
    await tester.pump();
    expect(find.byKey(const ValueKey('game-2-139')), findsOneWidget);
    expect(tester.takeException(), isNull);
    tester.view.physicalSize = const Size(640, 320);
    await tester.pump();
    expect(find.byKey(const ValueKey('game-2-139')), findsOneWidget);
    expect(tester.takeException(), isNull);
    await tester.pumpWidget(const SizedBox.shrink());
    controller.dispose();
  });
}

class _FakeGameApi implements GameApi {
  _FakeGameApi({
    this.levels = const [GameLevel(level: 1, rows: 2, columns: 2, mines: 1)],
    this.clickResult = const GameCell(state: CellState.revealed, value: 0),
    this.clickStatus,
    this.hintKind = HintKind.safe,
    this.hintX = 1,
  });

  final List<GameLevel> levels;
  final GameCell clickResult;
  GameStatus? clickStatus;
  final HintKind hintKind;
  final int hintX;
  int _nextGameId = 1;
  int botSteps = 0;
  int hints = 0;
  bool failClick = false;
  bool failFlag = false;
  (int, int)? lastFlaggedCell;
  final Map<String, GameSnapshot> _games = {};

  @override
  Future<List<GameLevel>> getLevels() async => levels;

  @override
  Future<GameSnapshot> newGame(int level) async {
    final configuration = levels.firstWhere((item) => item.level == level);
    final id = 'game-${_nextGameId++}';
    final game = GameSnapshot(
      id: id,
      rows: configuration.rows,
      columns: configuration.columns,
      mines: configuration.mines,
      remainingMines: configuration.mines,
      status: GameStatus.playing,
      cells: List.generate(
        configuration.rows * configuration.columns,
        (_) => const GameCell(state: CellState.unknown),
      ),
    );
    _games[id] = game;
    return game;
  }

  @override
  Future<GameSnapshot> click(String gameId, int x, int y) async {
    if (failClick) throw StateError('Click failed');
    return _update(gameId, x, y, clickResult, status: clickStatus);
  }

  @override
  Future<GameSnapshot> toggleFlag(String gameId, int x, int y) async {
    if (failFlag) throw StateError('Flag failed');
    lastFlaggedCell = (x, y);
    final game = _games[gameId]!;
    final current = game.cells[y * game.columns + x];
    final flagging = current.state != CellState.flag;
    return _replace(
      game,
      cells: [
        for (var index = 0; index < game.cells.length; index++)
          index == y * game.columns + x
              ? GameCell(state: flagging ? CellState.flag : CellState.unknown)
              : game.cells[index],
      ],
      remainingMines: game.remainingMines + (flagging ? -1 : 1),
    );
  }

  @override
  Future<BotStepResult> botStep(String gameId) async {
    botSteps++;
    return BotStepResult(
      game: _update(
        gameId,
        1,
        0,
        const GameCell(state: CellState.revealed, value: 1),
      ),
      x: 1,
      y: 0,
    );
  }

  @override
  Future<GameHint> hint(String gameId) async {
    hints++;
    return GameHint(x: hintX, y: 0, kind: hintKind);
  }

  @override
  Future<void> deleteGame(String gameId) async {
    _games.remove(gameId);
  }

  GameSnapshot _update(
    String gameId,
    int x,
    int y,
    GameCell cell, {
    GameStatus? status,
  }) {
    final game = _games[gameId]!;
    final cells = [...game.cells];
    cells[y * game.columns + x] = cell;
    return _replace(game, cells: cells, status: status);
  }

  GameSnapshot _replace(
    GameSnapshot game, {
    required List<GameCell> cells,
    int? remainingMines,
    GameStatus? status,
  }) {
    final updated = GameSnapshot(
      id: game.id,
      rows: game.rows,
      columns: game.columns,
      mines: game.mines,
      remainingMines: remainingMines ?? game.remainingMines,
      status: status ?? game.status,
      cells: cells,
    );
    _games[game.id] = updated;
    return updated;
  }
}

List<MethodCall> _mockWindowResizing() {
  final calls = <MethodCall>[];
  TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
      .setMockMethodCallHandler(_windowChannel, (call) async {
        calls.add(call);
        return null;
      });
  return calls;
}

void _clearWindowResizingMock() {
  TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
      .setMockMethodCallHandler(_windowChannel, null);
}
