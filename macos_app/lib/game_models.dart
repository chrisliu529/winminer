enum CellState { unknown, revealed, flag, exploded, mine }

enum GameStatus { playing, won, lost }

enum ActionSound { reveal, flag, mine }

class BotStepResult {
  const BotStepResult({required this.game, required this.x, required this.y});

  final GameSnapshot game;
  final int x;
  final int y;
}

class GameLevel {
  const GameLevel({
    required this.level,
    required this.rows,
    required this.columns,
    required this.mines,
  });

  final int level;
  final int rows;
  final int columns;
  final int mines;

  factory GameLevel.fromJson(Map<String, dynamic> json) => GameLevel(
    level: json['level'] as int,
    rows: json['rows'] as int,
    columns: json['columns'] as int,
    mines: json['mines'] as int,
  );
}

class GameCell {
  const GameCell({required this.state, this.value});

  final CellState state;
  final int? value;

  factory GameCell.fromJson(Map<String, dynamic> json) {
    final state = switch (json['state']) {
      'unknown' => CellState.unknown,
      'revealed' => CellState.revealed,
      'flag' => CellState.flag,
      'exploded' => CellState.exploded,
      'mine' => CellState.mine,
      final value => throw FormatException('Unknown cell state: $value'),
    };
    return GameCell(state: state, value: json['value'] as int?);
  }
}

class GameSnapshot {
  const GameSnapshot({
    required this.id,
    required this.rows,
    required this.columns,
    required this.mines,
    required this.remainingMines,
    required this.status,
    required this.cells,
  });

  final String id;
  final int rows;
  final int columns;
  final int mines;
  final int remainingMines;
  final GameStatus status;
  final List<GameCell> cells;

  factory GameSnapshot.fromJson(Map<String, dynamic> json) {
    final status = switch (json['status']) {
      'playing' => GameStatus.playing,
      'won' => GameStatus.won,
      'lost' => GameStatus.lost,
      final value => throw FormatException('Unknown game status: $value'),
    };
    return GameSnapshot(
      id: json['id'] as String,
      rows: json['rows'] as int,
      columns: json['columns'] as int,
      mines: json['mines'] as int,
      remainingMines: json['remainingMines'] as int,
      status: status,
      cells: (json['cells'] as List<dynamic>)
          .map(
            (cell) => GameCell.fromJson(Map<String, dynamic>.from(cell as Map)),
          )
          .toList(growable: false),
    );
  }
}
