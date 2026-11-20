import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:tessivo_desktop/src/core/http_core_client.dart';
import 'package:tessivo_desktop/src/core/supervisor.dart';

class FakeProcess implements Process {
  FakeProcess({this.stderrText = '', bool exitImmediately = false}) {
    if (exitImmediately) _exit.complete(1);
  }

  final String stderrText;
  final _exit = Completer<int>();
  final killed = <ProcessSignal>[];
  bool ignoreTerm = false;

  @override
  Future<int> get exitCode => _exit.future;

  @override
  bool kill([ProcessSignal signal = ProcessSignal.sigterm]) {
    killed.add(signal);
    if (!(ignoreTerm && signal == ProcessSignal.sigterm) && !_exit.isCompleted) _exit.complete(0);
    return true;
  }

  @override
  int get pid => 4242;

  @override
  Stream<List<int>> get stdout => const Stream<List<int>>.empty();

  @override
  Stream<List<int>> get stderr => Stream.value(utf8.encode(stderrText));

  @override
  IOSink get stdin => throw UnimplementedError();
}

const ep = Endpoint(network: 'tcp', address: '127.0.0.1:1', token: 't');

void main() {
  test('reuses a healthy running core without starting one', () async {
    var started = 0;
    final s = CoreSupervisor(
      executable: 'tess',
      discover: () async => ep,
      healthy: (_) async => true,
      start: (_, _) async {
        started++;
        return FakeProcess();
      },
    );
    expect(await s.ensureRunning(), ep);
    expect(started, 0);
    expect(s.ownsProcess, isFalse);
    await s.stop(); // no-op
  });

  test('starts tess serve and waits for a healthy endpoint', () async {
    var polls = 0;
    final proc = FakeProcess();
    late List<String> args;
    final s = CoreSupervisor(
      executable: '/opt/tess',
      discover: () async => ++polls > 2 ? ep : null,
      healthy: (_) async => true,
      start: (_, a) async {
        args = a;
        return proc;
      },
      pollInterval: Duration.zero,
    );
    expect(await s.ensureRunning(), ep);
    expect(args, ['serve', '--exit-with-parent']);
    expect(s.ownsProcess, isTrue);
    await s.stop();
    expect(proc.killed, [ProcessSignal.sigterm]);
  });

  test('stop escalates to SIGKILL when the core ignores SIGTERM', () async {
    final proc = FakeProcess()..ignoreTerm = true;
    var polls = 0;
    final s = CoreSupervisor(
      executable: 'tess',
      discover: () async => ++polls > 1 ? ep : null,
      healthy: (_) async => true,
      start: (_, _) async => proc,
      pollInterval: Duration.zero,
    );
    await s.ensureRunning();
    await s.stop(grace: const Duration(milliseconds: 10));
    expect(proc.killed, [ProcessSignal.sigterm, ProcessSignal.sigkill]);
  });

  test('reports a core that exits during startup', () async {
    final s = CoreSupervisor(
      executable: 'tess',
      discover: () async => null,
      start: (_, _) async => FakeProcess(stderrText: 'tess: no config', exitImmediately: true),
      pollInterval: const Duration(milliseconds: 5),
    );
    await expectLater(
      s.ensureRunning(),
      throwsA(isA<CoreStartException>().having((e) => e.message, 'message', contains('no config'))),
    );
  });

  test('times out, stops the process and ignores stale endpoint files', () async {
    final proc = FakeProcess();
    final s = CoreSupervisor(
      executable: 'tess',
      discover: () async => throw const FormatException('half written'),
      start: (_, _) async => proc,
      startTimeout: const Duration(milliseconds: 30),
      pollInterval: const Duration(milliseconds: 5),
    );
    await expectLater(s.ensureRunning(), throwsA(isA<CoreStartException>()));
    expect(proc.killed, isNotEmpty);
  });

  test('missing executable is a CoreStartException', () async {
    final s = CoreSupervisor(
      executable: 'tess',
      discover: () async => null,
      start: (exe, _) async => throw ProcessException(exe, const [], 'not found'),
    );
    await expectLater(s.ensureRunning(), throwsA(isA<CoreStartException>()));
  });

  test('unhealthy endpoint triggers a start; default health check fails closed', () async {
    var started = false;
    final s = CoreSupervisor(
      executable: 'tess',
      discover: () async => started ? ep : ep,
      start: (_, _) async {
        started = true;
        return FakeProcess(exitImmediately: true);
      },
      pollInterval: Duration.zero,
    );
    await expectLater(s.ensureRunning(), throwsA(isA<CoreStartException>()));
    expect(started, isTrue);
  });

  test('finds the bundled binary or falls back to PATH', () async {
    final dir = await Directory.systemTemp.createTemp('tsv');
    expect(findTessExecutable(resolvedExecutable: '${dir.path}/app', isWindows: false), 'tess');
    await File('${dir.path}/tess').writeAsString('');
    expect(findTessExecutable(resolvedExecutable: '${dir.path}/app', isWindows: false), '${dir.path}/tess');
    expect(findTessExecutable(resolvedExecutable: r'C:\nope\app.exe', isWindows: true), 'tess.exe');
    await dir.delete(recursive: true);
  });
}
