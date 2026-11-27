import 'dart:convert';
import 'dart:ffi';
import 'dart:io';
import 'dart:isolate';

import 'package:ffi/ffi.dart';

import 'game_models.dart';

abstract interface class GameApi {
  Future<List<GameLevel>> getLevels();
  Future<GameSnapshot> newGame(int level);
  Future<GameSnapshot> click(String gameId, int x, int y);
  Future<GameSnapshot> toggleFlag(String gameId, int x, int y);
  Future<GameSnapshot> botStep(String gameId);
  Future<void> deleteGame(String gameId);
}

class GoGameApi implements GameApi {
  String get _libraryPath {
    if (Platform.isAndroid) return 'libwinminer.so';
    if (!Platform.isMacOS) {
      throw UnsupportedError('GoGameApi is unavailable on this platform');
    }
    final executableDirectory = File(Platform.resolvedExecutable).parent.path;
    return '$executableDirectory/../Frameworks/libwinminer.dylib';
  }

  Future<Map<String, dynamic>> _call(Map<String, Object?> request) async {
    final libraryPath = _libraryPath;
    final payload = jsonEncode(request);
    final response = await Isolate.run<String>(
      () => _invokeGo(libraryPath, payload),
    );
    final decoded = jsonDecode(response);
    if (decoded is! Map) {
      throw const FormatException('Go bridge returned a non-object response');
    }
    final result = Map<String, dynamic>.from(decoded);
    final error = result['error'];
    if (error is String) {
      throw GoGameException(error);
    }
    return result;
  }

  @override
  Future<List<GameLevel>> getLevels() async {
    final response = await _call({'action': 'getLevels'});
    final levels = response['levels'];
    if (levels is! List) {
      throw const FormatException('Go bridge response is missing levels');
    }
    return levels
        .map(
          (level) =>
              GameLevel.fromJson(Map<String, dynamic>.from(level as Map)),
        )
        .toList(growable: false);
  }

  @override
  Future<GameSnapshot> newGame(int level) async {
    return _stateFrom(await _call({'action': 'newGame', 'level': level}));
  }

  @override
  Future<GameSnapshot> click(String gameId, int x, int y) async {
    return _stateFrom(
      await _call({'action': 'click', 'gameId': gameId, 'x': x, 'y': y}),
    );
  }

  @override
  Future<GameSnapshot> toggleFlag(String gameId, int x, int y) async {
    return _stateFrom(
      await _call({'action': 'toggleFlag', 'gameId': gameId, 'x': x, 'y': y}),
    );
  }

  @override
  Future<GameSnapshot> botStep(String gameId) async {
    return _stateFrom(await _call({'action': 'botStep', 'gameId': gameId}));
  }

  @override
  Future<void> deleteGame(String gameId) async {
    await _call({'action': 'deleteGame', 'gameId': gameId});
  }

  GameSnapshot _stateFrom(Map<String, dynamic> response) {
    final state = response['state'];
    if (state is! Map) {
      throw const FormatException('Go bridge response is missing game state');
    }
    return GameSnapshot.fromJson(Map<String, dynamic>.from(state));
  }
}

class GoGameException implements Exception {
  const GoGameException(this.message);

  final String message;

  @override
  String toString() => message;
}

typedef _NativeCall = Pointer<Utf8> Function(Pointer<Utf8>);
typedef _DartCall = Pointer<Utf8> Function(Pointer<Utf8>);
typedef _NativeFree = Void Function(Pointer<Utf8>);
typedef _DartFree = void Function(Pointer<Utf8>);

String _invokeGo(String libraryPath, String request) {
  final library = DynamicLibrary.open(libraryPath);
  final call = library.lookupFunction<_NativeCall, _DartCall>('WinminerCall');
  final free = library.lookupFunction<_NativeFree, _DartFree>('WinminerFree');
  final input = request.toNativeUtf8();
  try {
    final output = call(input);
    if (output.address == 0) {
      throw StateError('Go bridge returned a null response');
    }
    try {
      return output.toDartString();
    } finally {
      free(output);
    }
  } finally {
    malloc.free(input);
  }
}
