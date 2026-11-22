import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:tessivo_desktop/main.dart';
import 'package:tessivo_desktop/src/app_state.dart';
import 'package:tessivo_desktop/src/core/core_client.dart';
import 'package:tessivo_desktop/src/core/http_core_client.dart';
import 'package:tessivo_desktop/src/core/models.dart';

import 'support/real_http.dart';

/// Records answers and can pause a run, on top of the demo core.
class RecordingCore extends DemoCoreClient {
  RecordingCore() : super(tick: Duration.zero);

  final answers = <String>[];
  final resumed = <String>[];
  bool pauseRuns = false;
  bool failAnswers = false;
  bool interrupted = false;

  @override
  Future<List<PendingApproval>> pendingApprovals() async => [
    ...await super.pendingApprovals(),
    if (interrupted)
      const PendingApproval(runId: 'r-int', approvalId: '', tool: '', notebookName: 'Ops', status: 'interrupted'),
  ];

  @override
  Future<void> resumeRun(String runId) async {
    resumed.add(runId);
    interrupted = false;
  }

  @override
  Future<void> answerApproval(String runId, String approvalId, {required bool allow, String? remember}) {
    if (failAnswers) throw const CoreException(409, 'approval expired');
    answers.add('$approvalId:$allow:${remember ?? ''}');
    return super.answerApproval(runId, approvalId, allow: allow, remember: remember);
  }

  @override
  Stream<RunEvent> startRun({
    required String notebookId,
    required String prompt,
    required Map<String, String> vars,
    String? promptName,
  }) {
    if (!pauseRuns) return super.startRun(notebookId: notebookId, prompt: prompt, vars: vars, promptName: promptName);
    return Stream.fromIterable(const [
      RunToolCall(tool: 'read_file', args: '{"path":"a.go"}'),
      RunToolResult(tool: 'read_file', result: 'package a'),
      RunPaused(
        runId: 'r9',
        approvals: [
          PendingApproval(
            runId: 'r9',
            notebookId: 'nb-1',
            notebookName: 'Change risk review',
            approvalId: 'a9',
            tool: 'run_command',
            args: '{"program":"go"}',
            requestedAt: '2026-11-21T10:00:00Z',
            expiresAt: '2026-11-28T10:00:00Z',
          ),
        ],
      ),
    ]);
  }
}

Future<(AppState, RecordingCore)> pumpWith(WidgetTester tester) async {
  tester.view.physicalSize = const Size(1600, 1000);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  final core = RecordingCore();
  final state = AppState(core);
  await tester.pumpWidget(TessivoApp(state: state));
  await state.load();
  await tester.pumpAndSettle();
  return (state, core);
}

void main() {
  moreTests();
  testWidgets('approvals badge shows count', (tester) async {
    final (state, _) = await pumpWith(tester);
    expect(state.approvals, hasLength(1));
    expect(find.byKey(const Key('approvals-badge')), findsOneWidget);
    expect(find.text('Approvals (1)'), findsOneWidget);
  });

  testWidgets('allow once calls answerApproval and removes the item', (tester) async {
    final (state, core) = await pumpWith(tester);
    await tester.tap(find.byKey(const Key('approvals-badge')));
    await tester.pumpAndSettle();
    expect(find.text('grep'), findsWidgets);
    await tester.tap(find.byKey(const Key('approve-once')).first);
    await tester.pumpAndSettle();
    expect(core.answers.single, endsWith(':true:'));
    expect(state.approvals, isEmpty);
    expect(find.byKey(const Key('approvals-badge')), findsNothing);
  });

  testWidgets('deny sends allow false', (tester) async {
    final (_, core) = await pumpWith(tester);
    await tester.tap(find.byKey(const Key('approvals-badge')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('deny')).first);
    await tester.pumpAndSettle();
    expect(core.answers.single, endsWith(':false:'));
  });

  testWidgets('allow for this notebook sends remember', (tester) async {
    final (_, core) = await pumpWith(tester);
    await tester.tap(find.byKey(const Key('approvals-badge')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('approve-notebook')).first);
    await tester.pumpAndSettle();
    expect(core.answers.single, endsWith(':true:notebook'));
  });

  testWidgets('paused run shows waiting state', (tester) async {
    final (state, core) = await pumpWith(tester);
    core.pauseRuns = true;
    await tester.tap(find.byKey(const Key('run')));
    await tester.pumpAndSettle();
    expect(state.running, isFalse);
    expect(state.paused, hasLength(1));
    expect(find.text('Waiting for approval'), findsOneWidget);
    expect(find.textContaining('read_file'), findsWidgets);
    await tester.tap(find.byKey(const Key('approve-run')).first);
    await tester.pumpAndSettle();
    expect(core.answers.single, 'a9:true:run');
    expect(state.paused, isEmpty);
  });

  test('approval-needed events have a message', () {
    const e = CoreEvent(kind: 'run.approval_needed', notebookName: 'Ops', tool: 'grep');
    expect(e.message, 'Ops: approval needed for grep');
    expect(const CoreEvent(kind: 'run.failed', notebookName: 'Ops').message, contains('failed'));
  });

  group('HTTP', () {
    late HttpServer server;
    late HttpCoreClient client;
    final seen = <String>[];

    setUp(() async {
      seen.clear();
      server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
      server.listen((req) async {
        final body = await utf8.decoder.bind(req).join();
        seen.add('${req.method} ${req.uri.path} $body');
        final res = req.response;
        if (req.uri.path == '/v1/approvals') {
          res.headers.contentType = ContentType.json;
          res.write(
            jsonEncode({
              'approvals': [
                {'runId': 'r1', 'approvalId': 'a1', 'tool': 'grep', 'args': '{}', 'notebookName': 'Repo'},
              ],
            }),
          );
        } else if (req.uri.path.endsWith('/runs')) {
          res.headers.contentType = ContentType('text', 'event-stream');
          res.write('event: tool_call\ndata: {"text":"grep","tool":"grep","args":"{}"}\n\n');
          res.write('event: tool_result\ndata: {"text":"grep","tool":"grep","result":"a.go:1: x"}\n\n');
          res.write('event: approval_needed\ndata: {"text":"x"}\n\n');
          res.write(
            'event: paused\ndata: {"runId":"r1","approvals":[{"runId":"r1","approvalId":"a1","tool":"x"}]}\n\n',
          );
        } else if (req.uri.path.endsWith('/cancel')) {
          res.statusCode = 204;
        } else {
          res.statusCode = 202;
          res.headers.contentType = ContentType.json;
          res.write('{"status":"resuming"}');
        }
        await res.close();
      });
      client = HttpCoreClient(
        Endpoint(network: 'tcp', address: '127.0.0.1:${server.port}', token: 't'),
        client: realHttpClient(),
      );
    });

    tearDown(() async {
      client.close();
      await server.close(force: true);
    });

    test('lists, answers and cancels approvals', () async {
      final list = await client.pendingApprovals();
      expect(list.single.tool, 'grep');
      expect(list.single.notebookName, 'Repo');
      await client.answerApproval('r1', 'a1', allow: false, remember: 'run');
      await client.cancelRun('r1');
      expect(seen[1], contains('POST /v1/runs/r1/approvals/a1'));
      expect(seen[1], contains('"decision":"deny"'));
      expect(seen[1], contains('"remember":"run"'));
      expect(seen[2], startsWith('POST /v1/runs/r1/cancel'));
    });

    test('parses agent run events', () async {
      final events = await client.startRun(notebookId: 'repo', prompt: 'p', vars: const {}).toList();
      expect((events[0] as RunToolCall).tool, 'grep');
      expect((events[1] as RunToolResult).result, 'a.go:1: x');
      final paused = events.last as RunPaused;
      expect(paused.runId, 'r1');
      expect(paused.approvals.single.approvalId, 'a1');
    });
  });
}

void moreTests() {
  testWidgets('interrupted runs can be resumed from the sheet', (tester) async {
    final (state, core) = await pumpWith(tester);
    core.interrupted = true;
    await state.refreshApprovals();
    await tester.pumpAndSettle();
    expect(find.text('Approvals (2)'), findsOneWidget);
    await tester.tap(find.byKey(const Key('approvals-badge')));
    await tester.pumpAndSettle();
    expect(find.textContaining('Interrupted'), findsOneWidget);
    await tester.tap(find.byKey(const Key('resume-run')));
    await tester.pumpAndSettle();
    expect(core.resumed, ['r-int']);
  });

  testWidgets('a failed answer shows a notice', (tester) async {
    final (state, core) = await pumpWith(tester);
    core.failAnswers = true;
    await tester.tap(find.byKey(const Key('approvals-badge')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('approve-once')).first);
    await tester.pumpAndSettle();
    expect(state.notices.last, contains('approval expired'));
  });
}
