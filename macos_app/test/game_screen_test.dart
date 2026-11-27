import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:winminer_app/game_api.dart';
import 'package:winminer_app/game_models.dart';
import 'package:winminer_app/game_screen.dart';

const _windowChannel = MethodChannel('winminer/window');

void main() {
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
}

class _FakeGameApi implements GameApi {
  _FakeGameApi({
    this.levels = const [GameLevel(level: 1, rows: 2, columns: 2, mines: 1)],
  });

  final List<GameLevel> levels;
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
    return _update(
      gameId,
      x,
      y,
      const GameCell(state: CellState.revealed, value: 0),
    );
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

  GameSnapshot _update(String gameId, int x, int y, GameCell cell) {
    final game = _games[gameId]!;
    final cells = [...game.cells];
    cells[y * game.columns + x] = cell;
    return _replace(game, cells: cells);
  }

  GameSnapshot _replace(
    GameSnapshot game, {
    required List<GameCell> cells,
    int? remainingMines,
  }) {
    final updated = GameSnapshot(
      id: game.id,
      rows: game.rows,
      columns: game.columns,
      mines: game.mines,
      remainingMines: remainingMines ?? game.remainingMines,
      status: game.status,
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
