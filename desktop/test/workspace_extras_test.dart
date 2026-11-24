import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:tessivo_desktop/main.dart';
import 'package:tessivo_desktop/src/app_state.dart';
import 'package:tessivo_desktop/src/core/http_core_client.dart';
import 'package:tessivo_desktop/src/core/models.dart';

import 'support/fake_core/fake_core.dart';
import 'support/real_http.dart';

const token = 'secret-token';

const settingsJson = {
  'providers': [
    {
      'name': 'litellm',
      'base_url': 'https://llm.internal/v1',
      'api_key_ref': 'KEY_REF',
      'headers': {'X-Team': 'ops'},
    },
  ],
  'aliases': [
    {
      'name': 'smart',
      'chain': [
        {'provider': 'litellm', 'model': 'a'},
        {'provider': 'litellm'},
      ],
      'fallback_on': ['timeout', 42],
    },
  ],
  'defaults': {'model': 'smart', 'thinking_effort': 'high', 'max_output_tokens': 4096.0},
  'limits': {'max_tokens_per_day': 100000},
};

/// Fake of the core's notebook-detail, context, memory, output and settings endpoints.
Future<HttpServer> fakeExtras() async {
  final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
  server.listen((req) async {
    final res = req.response;
    final text = await utf8.decoder.bind(req).join();
    final body = text.isEmpty ? const <String, Object?>{} : jsonDecode(text) as Map<String, Object?>;
    final path = req.uri.path;
    Map<String, Object?> nb({List<Object?> memory = const [], List<Object?> context = const []}) => {
      'id': 'board',
      'name': 'Board',
      'workType': 'analyze',
      'memory': memory,
      'context': context,
    };
    if (req.method == 'GET' && path == '/v1/notebooks/board') {
      res.write(jsonEncode(nb(memory: ['Audience is the CFO', 7])));
    } else if (req.method == 'POST' && path == '/v1/notebooks/board/context') {
      res.write(
        jsonEncode(
          nb(
            context: [
              {'id': 'c9', 'label': body['path'], 'enabled': true, 'inPlace': true, 'estimatedTokens': 5000},
            ],
          ),
        ),
      );
    } else if (req.method == 'PUT' && path == '/v1/notebooks/board/memory') {
      res.write(jsonEncode(nb(memory: body['memory']! as List<Object?>)));
    } else if (path == '/v1/notebooks/board/outputs/o1') {
      res.write(jsonEncode({'id': 'o1', 'title': 'Digest', 'markdown': '# Digest'}));
    } else if (path == '/v1/settings') {
      res.write(jsonEncode(settingsJson));
    } else if (path == '/v1/models') {
      final all = {
        'litellm': ['z-model', 'a-model'],
        'bad': 'nope',
      };
      final p = req.uri.queryParameters['provider'];
      res.write(
        jsonEncode({
          'models': p == null ? all : {p: all[p]},
        }),
      );
    } else {
      res.statusCode = 404;
      res.write(jsonEncode({'error': 'not found'}));
    }
    await res.close();
  });
  return server;
}

Future<AppState> pumpWorkspace(WidgetTester tester) async {
  tester.view.physicalSize = const Size(1600, 1400);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  final state = AppState(FakeCoreClient(tick: Duration.zero));
  await tester.pumpWidget(TessivoApp(state: state));
  await state.load();
  await tester.pumpAndSettle();
  return state;
}

Future<void> answer(WidgetTester tester, String value) async {
  await tester.enterText(find.byKey(const Key('text-prompt')), value);
  await tester.tap(find.byKey(const Key('text-prompt-ok')));
  await tester.pumpAndSettle();
}

void main() {
  group('http client', () {
    late HttpServer server;
    late HttpCoreClient client;

    setUp(() async {
      server = await fakeExtras();
      client = HttpCoreClient(
        Endpoint(network: 'tcp', address: '127.0.0.1:${server.port}', token: token),
        client: realHttpClient(),
      );
    });
    tearDown(() async {
      client.close();
      await server.close(force: true);
    });

    test('notebook detail keeps only string memory', () async {
      expect((await client.getNotebook('board')).memory, ['Audience is the CFO']);
    });

    test('adds context and replaces memory', () async {
      final nb = await client.addContext('board', '/srv/repo');
      expect(nb.context.single.kind, 'Folder');
      expect((await client.setMemory('board', ['a', 'b'])).memory, ['a', 'b']);
    });

    test('validates before sending', () {
      expect(() => client.addContext('board', 'relative/path'), throwsFormatException);
      expect(() => client.setMemory('board', ['x' * (maxMemoryItemLength + 1)]), throwsFormatException);
    });

    test('opens an output', () async {
      expect((await client.getOutput('board', 'o1')).markdown, '# Digest');
      expect(() => client.getOutput('board', 'nope'), throwsA(isA<CoreException>()));
    });

    test('settings skip headers and malformed fields', () async {
      final s = await client.getSettings();
      expect(s.providers.single.keyRef, 'KEY_REF');
      expect(s.aliases.single.chain, ['litellm/a', 'litellm/?']);
      expect(s.aliases.single.fallbackOn, ['timeout']);
      expect(s.maxOutputTokens, 4096);
      expect(s.maxTokensPerRun, isNull);
      expect(s.maxTokensPerDay, 100000);
    });

    test('models are sorted and malformed providers are empty', () async {
      final all = await client.listModels();
      expect(all['litellm'], ['a-model', 'z-model']);
      expect(all['bad'], isEmpty);
      expect((await client.listModels(provider: 'litellm')).keys, ['litellm']);
    });
  });

  test('mapping tolerates missing objects', () {
    expect(settingsFromJson(const {}).providers, isEmpty);
    expect(modelsFromJson(const {'models': 'x'}), isEmpty);
  });

  test('path and memory checks', () {
    for (final ok in ['/a', r'C:\repo', 'D:/x', r'\\server\share']) {
      expect(() => checkContextPath(ok), returnsNormally, reason: ok);
    }
    for (final bad in ['', 'rel', ' /a', '~/a']) {
      expect(() => checkContextPath(bad), throwsFormatException, reason: bad);
    }
    expect(() => checkMemory(List.filled(maxMemoryItems + 1, 'x')), throwsFormatException);
    expect(() => checkMemory(['  ']), throwsFormatException);
    expect(() => checkMemory(['fine']), returnsNormally);
  });

  test('demo core covers the new calls', () async {
    final core = FakeCoreClient(tick: Duration.zero);
    expect(() => core.getNotebook('missing'), throwsStateError);
    expect(() => core.getOutput('nb-2', 'o1'), throwsStateError);
    expect((await core.getOutput('nb-1', 'o1')).markdown, startsWith('# PR #482'));
    expect(await core.listModels(provider: 'other'), isEmpty);
    final nb = await core.addContext('nb-2', '/Users/me/contracts/acme.pdf');
    expect(nb.context.single.kind, 'PDF');
  });

  testWidgets('adds context by path and reports bad paths', (tester) async {
    final state = await pumpWorkspace(tester);
    state.select('nb-2');
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const Key('context-add')));
    await tester.pumpAndSettle();
    await answer(tester, 'contracts');
    expect(find.textContaining('Use an absolute path'), findsOneWidget);

    await tester.tap(find.byKey(const Key('context-add')));
    await tester.pumpAndSettle();
    await answer(tester, '/Users/me/contracts');
    expect(find.text('contracts'), findsOneWidget);
    expect(state.selected!.context, hasLength(1));
  });

  testWidgets('memory facts can be added and forgotten', (tester) async {
    final state = await pumpWorkspace(tester);
    expect(find.text('0/$maxMemoryItems'), findsOneWidget);

    await tester.tap(find.byKey(const Key('memory-add')));
    await tester.pumpAndSettle();
    await answer(tester, 'Audience is the CFO');
    expect(find.text('Audience is the CFO'), findsOneWidget);
    expect(state.selected!.memory, ['Audience is the CFO']);

    await tester.tap(find.byKey(const Key('memory-remove-0')));
    await tester.pumpAndSettle();
    expect(state.selected!.memory, isEmpty);
  });

  testWidgets('memory errors show a message', (tester) async {
    final state = AppState(_RejectingCore());
    tester.view.physicalSize = const Size(1600, 1400);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(TessivoApp(state: state));
    await state.load();
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('memory-add')));
    await tester.pumpAndSettle();
    await answer(tester, 'anything');
    expect(find.textContaining('rejected'), findsOneWidget);
  });

  testWidgets('library opens an output', (tester) async {
    await pumpWorkspace(tester);
    await tester.tap(find.text('Outputs library'));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('output-o2')));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('output-markdown')), findsOneWidget);
    expect(find.textContaining('Sample output from Weekly ops report'), findsOneWidget);
    await tester.tap(find.text('Close'));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('output-markdown')), findsNothing);
  });

  testWidgets('library shows an error when an output cannot open', (tester) async {
    final state = AppState(_RejectingCore());
    tester.view.physicalSize = const Size(1600, 1400);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(TessivoApp(state: state));
    await state.load();
    state.go(Section.library);
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('output-o1')));
    await tester.pumpAndSettle();
    expect(find.textContaining('Could not open this output'), findsOneWidget);
  });
}

class _RejectingCore extends FakeCoreClient {
  @override
  Future<Notebook> getNotebook(String notebookId) => throw StateError('offline');

  @override
  Future<Notebook> setMemory(String notebookId, List<String> memory) => throw StateError('rejected');

  @override
  Future<OutputDetail> getOutput(String notebookId, String outputId) async => throw StateError('gone');
}
