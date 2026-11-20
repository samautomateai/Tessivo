import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'http_core_client.dart';

typedef ProcessStarter = Future<Process> Function(String executable, List<String> arguments);
typedef EndpointDiscovery = Future<Endpoint?> Function();
typedef HealthCheck = Future<bool> Function(Endpoint endpoint);

/// Thrown when the core cannot be started or never becomes healthy.
class CoreStartException implements Exception {
  const CoreStartException(this.message);
  final String message;

  @override
  String toString() => 'CoreStartException: $message';
}

/// Finds the `tess` binary: bundled next to the app first, then on PATH.
String findTessExecutable({String? resolvedExecutable, bool? isWindows}) {
  final win = isWindows ?? Platform.isWindows;
  final name = win ? 'tess.exe' : 'tess';
  final exe = resolvedExecutable ?? Platform.resolvedExecutable;
  final sep = win ? r'\' : '/';
  final dir = exe.substring(0, exe.lastIndexOf(sep) + 1);
  final bundled = '$dir$name';
  return File(bundled).existsSync() ? bundled : name;
}

/// Reuses a healthy running core or starts `tess serve` and waits for it.
/// Only a core this supervisor started is stopped by [stop].
class CoreSupervisor {
  CoreSupervisor({
    required this.executable,
    EndpointDiscovery? discover,
    ProcessStarter? start,
    HealthCheck? healthy,
    this.startTimeout = const Duration(seconds: 15),
    this.pollInterval = const Duration(milliseconds: 100),
  }) : _discover = discover ?? Endpoint.discover,
       _start = start ?? ((exe, args) => Process.start(exe, args)),
       _healthy = healthy ?? _checkHealth;

  final String executable;
  final Duration startTimeout;
  final Duration pollInterval;
  final EndpointDiscovery _discover;
  final ProcessStarter _start;
  final HealthCheck _healthy;

  Process? _process;
  final _stderrTail = StringBuffer();

  /// Whether this supervisor started (and owns) the core process.
  bool get ownsProcess => _process != null;

  static Future<bool> _checkHealth(Endpoint ep) async {
    final client = HttpCoreClient(ep);
    try {
      return await client.health().timeout(const Duration(seconds: 2));
    } on Object {
      return false;
    } finally {
      client.close();
    }
  }

  Future<Endpoint?> _healthyEndpoint() async {
    try {
      final ep = await _discover();
      if (ep != null && await _healthy(ep)) return ep;
    } on FormatException {
      // A half-written or stale endpoint file: keep waiting.
    }
    return null;
  }

  /// Returns a healthy endpoint, starting the core if needed.
  Future<Endpoint> ensureRunning() async {
    final existing = await _healthyEndpoint();
    if (existing != null) return existing;

    final Process proc;
    try {
      proc = await _start(executable, const ['serve', '--exit-with-parent']);
    } on ProcessException catch (e) {
      throw CoreStartException('could not start $executable: ${e.message}');
    }
    _process = proc;
    unawaited(proc.stdout.drain<void>());
    proc.stderr.transform(utf8.decoder).listen(_keepTail);
    var exited = false;
    unawaited(proc.exitCode.then((_) => exited = true));

    final deadline = DateTime.now().add(startTimeout);
    while (DateTime.now().isBefore(deadline)) {
      final ep = await _healthyEndpoint();
      if (ep != null) return ep;
      if (exited) {
        _process = null;
        throw CoreStartException('tess serve exited: ${_stderrTail.toString().trim()}');
      }
      await Future<void>.delayed(pollInterval);
    }
    await stop();
    throw CoreStartException('tess serve did not become ready within ${startTimeout.inSeconds}s');
  }

  void _keepTail(String chunk) {
    _stderrTail.write(chunk);
    if (_stderrTail.length > 4096) {
      final s = _stderrTail.toString();
      _stderrTail
        ..clear()
        ..write(s.substring(s.length - 4096));
    }
  }

  /// Stops the core if this supervisor started it: SIGTERM, then kill.
  Future<void> stop({Duration grace = const Duration(seconds: 5)}) async {
    final proc = _process;
    if (proc == null) return;
    _process = null;
    proc.kill();
    try {
      await proc.exitCode.timeout(grace);
    } on TimeoutException {
      proc.kill(ProcessSignal.sigkill);
    }
  }
}
