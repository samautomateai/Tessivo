import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:tessivo_desktop/src/app_state.dart';
import 'package:tessivo_desktop/src/core/http_core_client.dart';
import 'package:tessivo_desktop/src/core/models.dart';

import 'app_test.dart' show pumpApp;
import 'notebook_page_layout_test.dart' show pumpNotebook;
import 'support/fake_core/fake_core.dart';
import 'support/notebook_tabs.dart';
import 'support/real_http.dart';

String _box(WidgetTester tester) => tester.widget<TextField>(find.byKey(const Key('prompt'))).controller!.text;

Future<void> _use(WidgetTester tester, String name) async {
  await tester.tap(find.byKey(const Key('prompts-menu')));
  await tester.pumpAndSettle();
  await tester.tap(find.byKey(Key('prompt-use-$name')).last);
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('a saved prompt shows its version and restores an earlier one', (tester) async {
    final state = await pumpApp(tester);
    await state.savePromptAs('risk', 'first text');
    await state.savePromptAs('risk', 'second text');
    await openTab(tester, 'prompt');
    await _use(tester, 'risk');
    expect(find.text('Version 2'), findsOneWidget);

    await tester.tap(find.byKey(const Key('prompt-history')));
    await tester.pumpAndSettle();
    expect(find.text('Version 2 (current)'), findsOneWidget);
    expect(find.byKey(const Key('prompt-restore-2')), findsNothing); // the current one has no Restore
    await tester.tap(find.byKey(const Key('prompt-restore-1')));
    await tester.pumpAndSettle();
    expect(_box(tester), 'first text');
    expect(find.text('Version 3'), findsOneWidget); // a restore is a new save
    expect(find.text('Saved prompt: risk'), findsOneWidget);
  });

  testWidgets('closing the history restores nothing', (tester) async {
    final state = await pumpApp(tester);
    await state.savePromptAs('risk', 'only text');
    await openTab(tester, 'prompt');
    await _use(tester, 'risk');
    await tester.tap(find.byKey(const Key('prompt-history')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Close'));
    await tester.pumpAndSettle();
    expect(find.text('Version 1'), findsOneWidget);
  });

  testWidgets('read each source on its own: a run override, saved with a prompt', (tester) async {
    final state = await pumpApp(tester);
    final core = state.core as FakeCoreClient;
    await openTab(tester, 'prompt');
    await tester.tap(find.byKey(const Key('chain-per-source')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('run')));
    await tester.pumpAndSettle();
    expect(core.lastRunChain, chainPerSource);

    await openTab(tester, 'prompt');
    await tester.tap(find.byKey(const Key('prompts-menu')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('prompt-save')).last);
    await tester.pumpAndSettle();
    await tester.enterText(find.byType(TextField).last, 'deep');
    await tester.tap(find.text('OK'));
    await tester.pumpAndSettle();
    expect(state.prompts.single.chain, chainPerSource);

    await tester.tap(find.byKey(const Key('chain-per-source'))); // off again
    await tester.pumpAndSettle();
    await _use(tester, 'deep');
    expect(tester.widget<Checkbox>(find.byType(Checkbox).first).value, isTrue); // the prompt brings its setting
    await tester.tap(find.byKey(const Key('chain-per-source')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('run')));
    await tester.pumpAndSettle();
    expect(core.lastRunChain, isNull);
  });

  testWidgets('the Sources tab sets the watch mode', (tester) async {
    final state = await pumpApp(tester);
    await openTab(tester, 'sources');
    await tester.ensureVisible(find.byKey(const Key('watch-mode')));
    await tester.tap(find.byKey(const Key('watch-mode')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Tell me and run the notebook').last);
    await tester.pumpAndSettle();
    expect(state.selected!.watch, 'run');
    await tester.tap(find.byKey(const Key('watch-mode')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Do nothing').last);
    await tester.pumpAndSettle();
    expect(state.selected!.watch, '');
  });

  testWidgets('at 200 % zoom the Prompt tab with a saved prompt does not overflow', (tester) async {
    final state = await pumpNotebook(tester, size: const Size(1366, 768));
    await state.savePromptAs('a-rather-long-prompt-name', 'text');
    await state.setZoom(2.0);
    await tester.pumpAndSettle();
    await openTab(tester, 'prompt');
    await _use(tester, 'a-rather-long-prompt-name');
    expect(tester.takeException(), isNull);
    expect(find.byKey(const Key('prompt-history')), findsOneWidget);
  });

  test('a sources.changed event reads as a notice', () {
    CoreEvent e({bool ran = false, String? error}) =>
        CoreEvent(kind: 'sources.changed', notebookName: 'Ops', ran: ran, error: error);
    expect(e().message, 'Ops: sources changed');
    expect(e(ran: true).message, 'Ops: sources changed, a run started');
    expect(e(error: 'busy').message, 'Ops: sources changed (busy)');
  });

  test('prompt JSON carries its version and chain', () {
    final p = SavedPrompt.fromJson({
      'name': 'risk',
      'text': 'Assess',
      'version': 3,
      'settings': {'chain': 'per_source'},
    });
    expect((p.version, p.chain), (3, chainPerSource));
    expect(p.toBody(), {
      'text': 'Assess',
      'settings': {'chain': 'per_source'},
    });
    final v = PromptVersion.fromJson({'version': 2, 'text': 't', 'savedAt': '2026-11-25T10:00:00Z'});
    expect((v.version, v.text, v.savedAt), (2, 't', '2026-11-25T10:00:00Z'));
  });

  test('http client: prompt versions, restore, watch and chain', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    final calls = <String>[];
    server.listen((req) async {
      calls.add('${req.method} ${req.uri.path} ${await utf8.decoder.bind(req).join()}');
      final res = req.response;
      final path = req.uri.path;
      if (path.endsWith('/versions')) {
        res.write(
          jsonEncode({
            'versions': [
              {'version': 2, 'text': 'new'},
              {'version': 1, 'text': 'old', 'savedAt': '2026-11-24T09:00:00Z'},
            ],
          }),
        );
      } else if (path.endsWith('/restore')) {
        res.write(jsonEncode({'name': 'risk', 'text': 'old', 'version': 3}));
      } else if (req.method == 'PATCH') {
        res.write(jsonEncode({'id': 'ops', 'name': 'Ops', 'workType': 'analyze', 'watch': 'notify'}));
      } else {
        res.headers.contentType = ContentType('text', 'event-stream');
        res.write('event: status\ndata: {"text":"ok"}\n\n');
      }
      await res.close();
    });
    final client = HttpCoreClient(
      Endpoint(network: 'tcp', address: '127.0.0.1:${server.port}', token: 't'),
      client: realHttpClient(),
    );
    final vs = await client.promptVersions('ops', 'risk');
    expect(vs.map((v) => v.version), [2, 1]);
    expect(calls.last, startsWith('GET /v1/notebooks/ops/prompts/risk/versions'));
    final p = await client.restorePromptVersion('ops', 'risk', 1);
    expect((p.text, p.version), ('old', 3));
    expect(calls.last, startsWith('POST /v1/notebooks/ops/prompts/risk/versions/1/restore'));
    final nb = await client.updateNotebook('ops', watch: 'notify');
    expect(nb.watch, 'notify');
    expect(calls.last, 'PATCH /v1/notebooks/ops {"watch":"notify"}');
    await client.startRun(notebookId: 'ops', prompt: 'p', vars: const {}, chain: chainPerSource).toList();
    expect(calls.last, contains('"overrides":{"chain":"per_source"}'));
    await client.startRun(notebookId: 'ops', prompt: 'p', vars: const {}).toList();
    expect(calls.last, isNot(contains('overrides')));
    client.close();
    await server.close(force: true);
  });
}
