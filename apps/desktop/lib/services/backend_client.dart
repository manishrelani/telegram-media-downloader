import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter/foundation.dart' show kReleaseMode;

import '../models/backend_message.dart';

class BackendException implements Exception {
  const BackendException(this.code, this.message);
  final String code;
  final String message;

  @override
  String toString() => '$code: $message';
}

/// Owns one backend child process and communicates using JSON Lines v1.
/// Standard output is reserved for protocol messages; stderr is diagnostic only.
class BackendClient {
  BackendClient({String? executablePath})
    : _explicitExecutable = executablePath;

  final String? _explicitExecutable;
  final StreamController<BackendMessage> _events = StreamController.broadcast();
  final Map<String, Completer<BackendMessage>> _pending = {};
  final StringBuffer _stderrTail = StringBuffer();
  Process? _process;
  StreamSubscription<String>? _stdoutSubscription;
  StreamSubscription<String>? _stderrSubscription;
  bool _disposed = false;
  int _sequence = 0;

  Stream<BackendMessage> get events => _events.stream;
  bool get isRunning => _process != null;
  String get diagnostics => _stderrTail.toString();

  Future<void> start() async {
    if (_disposed) throw StateError('BackendClient has been disposed.');
    if (_process != null) return;
    final String executable = await _resolveExecutable();
    final Map<String, String> environment = Map<String, String>.from(
      Platform.environment,
    );
    final String? tdlPath = findTdlExecutable(environment, executable);
    if (tdlPath != null) environment['TDL_PATH'] = tdlPath;

    final Process process;
    try {
      process = await Process.start(
        executable,
        const <String>[],
        mode: ProcessStartMode.normal,
        runInShell: false,
        environment: environment,
      );
    } on ProcessException catch (error) {
      throw BackendException(
        'backend_start_failed',
        backendStartErrorMessage(
          executable: executable,
          error: error,
          isMacOS: Platform.isMacOS,
        ),
      );
    }
    _process = process;
    _stdoutSubscription = process.stdout
        .transform(utf8.decoder)
        .transform(const LineSplitter())
        .listen(
          _handleStdout,
          onError: (Object error) =>
              _failPending('backend_output_error', '$error'),
        );
    _stderrSubscription = process.stderr
        .transform(utf8.decoder)
        .transform(const LineSplitter())
        .listen(_handleStderr);
    unawaited(
      process.exitCode.then((int code) {
        _process = null;
        if (!_disposed) {
          _failPending(
            'backend_exited',
            'Backend exited with code $code. ${diagnostics.trim()}'.trim(),
          );
        }
      }),
    );
  }

  Future<BackendMessage> request(
    String action, {
    Map<String, dynamic>? params,
    String? requestId,
    Duration timeout = const Duration(seconds: 30),
  }) async {
    await start();
    final Process? process = _process;
    if (process == null) {
      throw const BackendException(
        'backend_not_running',
        'The backend process is not running.',
      );
    }
    final String id = requestId ?? _nextID(action);
    final Completer<BackendMessage> completer = Completer<BackendMessage>();
    _pending[id] = completer;
    final Map<String, dynamic> request = <String, dynamic>{
      'version': 1,
      'id': id,
      'action': action,
      if (params != null) 'params': params,
    };
    try {
      process.stdin.writeln(jsonEncode(request));
      await process.stdin.flush();
      return await completer.future.timeout(
        timeout,
        onTimeout: () {
          _pending.remove(id);
          throw TimeoutException(
            'Backend request "$action" timed out after ${timeout.inSeconds}s.',
          );
        },
      );
    } on Object {
      _pending.remove(id);
      rethrow;
    }
  }

  Future<void> dispose() async {
    if (_disposed) return;
    _disposed = true;
    final Process? process = _process;
    if (process != null) {
      final Future<int> exit = process.exitCode;
      try {
        // EOF lets the Go protocol loop stop and its handler drain active jobs.
        await process.stdin.close();
        await exit.timeout(const Duration(seconds: 12));
      } on TimeoutException {
        _terminate(process, force: false);
        try {
          await exit.timeout(const Duration(seconds: 2));
        } on TimeoutException {
          _terminate(process, force: true);
        }
      } on Object {
        _terminate(process, force: false);
      } finally {
        _process = null;
      }
    }
    // Keep both pipes drained until the backend exits: it may need to emit terminal
    // job events while persisting a graceful pause during shutdown.
    await _stdoutSubscription?.cancel();
    await _stderrSubscription?.cancel();
    _failPending('backend_disposed', 'Backend connection was closed.');
    await _events.close();
  }

  void _handleStdout(String line) {
    if (line.trim().isEmpty) return;
    try {
      final dynamic decoded = jsonDecode(line);
      if (decoded is! Map<String, dynamic>) {
        _failPending(
          'invalid_backend_message',
          'Backend emitted a non-object JSON line.',
        );
        return;
      }
      final BackendMessage message = BackendMessage.fromJson(decoded);
      if (!_events.isClosed) _events.add(message);
      final Completer<BackendMessage>? completer = _pending[message.id];
      if (completer != null &&
          (message.event == 'result' ||
              message.event == 'job_accepted' ||
              message.isError)) {
        _pending.remove(message.id);
        if (message.isError) {
          completer.completeError(
            BackendException(
              message.errorCode ?? 'backend_error',
              message.errorMessage ??
                  'The backend reported an unspecified error.',
            ),
          );
        } else {
          completer.complete(message);
        }
      }
    } on FormatException catch (error) {
      _failPending(
        'invalid_backend_json',
        'Could not parse backend output: ${error.message}',
      );
    }
  }

  void _handleStderr(String line) {
    if (line.trim().isEmpty) return;
    _stderrTail.writeln(line);
    final String current = _stderrTail.toString();
    if (current.length > 8000) {
      _stderrTail
        ..clear()
        ..write(current.substring(current.length - 6000));
    }
  }

  void _failPending(String code, String message) {
    final List<Completer<BackendMessage>> pending = _pending.values.toList();
    _pending.clear();
    for (final Completer<BackendMessage> completer in pending) {
      if (!completer.isCompleted) {
        completer.completeError(BackendException(code, message));
      }
    }
  }

  void _terminate(Process process, {required bool force}) {
    try {
      process.kill(force ? ProcessSignal.sigkill : ProcessSignal.sigterm);
    } on Object {
      // Windows has a different process termination model; default kill is the fallback.
      process.kill();
    }
  }

  String _nextID(String prefix) =>
      '$prefix-${DateTime.now().microsecondsSinceEpoch}-${_sequence++}';

  Future<String> _resolveExecutable() => findBackendExecutable(
    environment: Platform.environment,
    appExecutable: Platform.resolvedExecutable,
    workingDirectory: Directory.current.path,
    explicitPath: _explicitExecutable,
  );
}

String backendStartErrorMessage({
  required String executable,
  required ProcessException error,
  required bool isMacOS,
}) {
  final String message =
      'Could not start the Go backend at "$executable": ${error.message}';
  if (isMacOS &&
      (error.errorCode == 1 ||
          error.message.toLowerCase().contains('operation not permitted'))) {
    return '$message. App Sandbox may be blocking the helper. For this local build, run python3 scripts/configure_macos_entitlements.py and rebuild. This configuration is for distribution outside the Mac App Store.';
  }
  return message;
}

String? findTdlExecutable(
  Map<String, String> environment,
  String backendExecutable, {
  bool releaseMode = kReleaseMode,
}) {
  final configured = environment['TDL_PATH'];
  if (!releaseMode && configured != null && configured.trim().isNotEmpty) {
    return configured;
  }
  final name = Platform.isWindows ? 'tdl-tmm.exe' : 'tdl-tmm';
  final bundled = File(
    '${File(backendExecutable).parent.path}${Platform.pathSeparator}$name',
  );
  if (bundled.existsSync()) return bundled.path;
  if (releaseMode) {
    throw const BackendException(
      'runtime_not_found',
      'The bundled Telegram runtime is missing. Reinstall the app.',
    );
  }
  throw const BackendException(
    'runtime_not_found',
    'The bundled Telegram runtime is missing. Reinstall the app or set a trusted TDL_PATH for development.',
  );
}

Future<String> findBackendExecutable({
  required Map<String, String> environment,
  required String appExecutable,
  required String workingDirectory,
  String? explicitPath,
  bool releaseMode = kReleaseMode,
}) async {
  final String executableName = Platform.isWindows
      ? 'telegram-media-downloader-backend.exe'
      : 'telegram-media-downloader-backend';
  final bundled = File(
    '${File(appExecutable).parent.path}${Platform.pathSeparator}$executableName',
  );
  if (releaseMode) {
    if (await bundled.exists()) return bundled.absolute.path;
    throw const BackendException(
      'backend_not_found',
      'The bundled backend is missing. Reinstall the app.',
    );
  }

  if (explicitPath != null && explicitPath.trim().isNotEmpty) {
    final File explicit = File(explicitPath);
    if (await explicit.exists()) return explicit.path;
    throw BackendException(
      'backend_not_found',
      'Backend executable not found: ${explicit.path}',
    );
  }
  final String? envPath = environment['TMM_BACKEND_PATH'];
  if (envPath != null && envPath.trim().isNotEmpty) {
    if (await File(envPath).exists()) return envPath;
    throw BackendException(
      'backend_not_found',
      'TMM_BACKEND_PATH does not point to a file: $envPath',
    );
  }
  final List<String> candidates = <String>[
    '${File(appExecutable).parent.path}${Platform.pathSeparator}$executableName',
    '$workingDirectory${Platform.pathSeparator}build${Platform.pathSeparator}$executableName',
    '$workingDirectory${Platform.pathSeparator}..${Platform.pathSeparator}build${Platform.pathSeparator}$executableName',
    '$workingDirectory${Platform.pathSeparator}..${Platform.pathSeparator}..${Platform.pathSeparator}build${Platform.pathSeparator}$executableName',
  ];
  for (final String candidate in candidates) {
    final File file = File(candidate);
    if (await file.exists()) return file.absolute.path;
  }
  throw BackendException(
    'backend_not_found',
    'Could not locate $executableName. Set TMM_BACKEND_PATH to the backend executable path.',
  );
}
