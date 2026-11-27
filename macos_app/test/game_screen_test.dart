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
      MaterialApp(home: GameScreen(api: api, mobile: true)),
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

  testWidgets('phone solver appears after inactivity and resets on new game', (
    tester,
  ) async {
    final api = _FakeGameApi();
    await tester.pumpWidget(
      MaterialApp(home: GameScreen(api: api, mobile: true)),
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
    await tester.longPress(find.byKey(const ValueKey('game-1-1')));
    await tester.pump();
    await tester.pump(const Duration(seconds: 4));
    expect(find.byKey(const ValueKey('bot-step')), findsNothing);
    await tester.pump(const Duration(seconds: 1));
    expect(find.byKey(const ValueKey('bot-step')), findsOneWidget);
    await tester.tap(find.byKey(const ValueKey('bot-step')));
    await tester.pump();
    expect(api.botSteps, 1);
    await tester.tap(find.byKey(const ValueKey('restart')));
    await tester.pump();
    expect(find.byKey(const ValueKey('bot-step')), findsNothing);
    expect(find.byKey(const ValueKey('game-2-0')), findsOneWidget);
    await tester.pumpWidget(const SizedBox.shrink());
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
          body: MobileGameScreen(controller: controller, now: () => now),
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
        ],
      ),
    );
    await controller.initialize();
    var now = DateTime(2026, 1, 1);
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: MobileGameScreen(controller: controller, now: () => now),
        ),
      ),
    );
    expect(find.byKey(const ValueKey('game-1-63')), findsOneWidget);
    await tester.tap(find.byKey(const ValueKey('level-2')));
    await tester.pump();
    expect(find.byKey(const ValueKey('game-2-99')), findsOneWidget);
    expect(
      tester.getSize(find.byKey(const ValueKey('mobile-board'))).width,
      greaterThan(250),
    );
    await tester.tap(find.byKey(const ValueKey('game-2-0')));
    await tester.pump();
    now = now.add(const Duration(seconds: 1002));
    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
    await tester.pump();
    expect(find.bySemanticsLabel('999'), findsOneWidget);
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
  });

  final List<GameLevel> levels;
  final GameCell clickResult;
  GameStatus? clickStatus;
  int _nextGameId = 1;
  int botSteps = 0;
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
    return _update(gameId, x, y, clickResult, status: clickStatus);
  }

  @override
  Future<GameSnapshot> toggleFlag(String gameId, int x, int y) async {
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
  Future<GameSnapshot> botStep(String gameId) async {
    botSteps++;
    return _update(
      gameId,
      1,
      0,
      const GameCell(state: CellState.revealed, value: 1),
    );
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
