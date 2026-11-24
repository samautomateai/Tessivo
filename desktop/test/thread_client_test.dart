import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:tessivo_desktop/src/core/http_core_client.dart';

import 'support/fake_core/fake_core.dart';
import 'support/real_http.dart';

const _token = 'secret-token';
const _out = '/v1/notebooks/board/outputs/20261121T100000-abcdef';

const _promptSuggestion = {
  'id': 's2',
  'kind': 'prompt',
  'text': 'Summarize. Cite files.',
  'reason': 'asked twice',
  'status': 'open',
  'target': {
    'kind': 'saved',
    'promptName': 'weekly',
    'schedules': ['a', 'b'],
    'current': 'Summarize.',
  },
};

/// Fake of the core's output-thread endpoints; records method, path and body.
Future<(HttpServer, List<String>)> _fakeThreads() async {
  final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
  final calls = <String>[];
  server.listen((req) async {
    final res = req.response;
    final text = await utf8.decoder.bind(req).join();
    calls.add('${req.method} ${req.uri.path} $text');
    switch ((req.method, req.uri.path)) {
      case ('GET', '$_out/thread') || ('PATCH', '$_out/thread'):
        res.write(
          jsonEncode({
            'id': '20261121T100000-abcdef',
            'title': '',
            'thinkingEffort': 'high',
            'messages': [
              {'id': 'm1', 'role': 'user', 'content': 'why?'},
              {'id': 'n1', 'role': 'note', 'content': 'Saved as v2'},
            ],
          }),
        );
      case ('POST', '$_out/thread/messages'):
        res.headers.contentType = ContentType('text', 'event-stream');
        res.write('event: delta\ndata: {"text":"Because"}\n\n');
        res.write('event: suggestion\ndata: ${jsonEncode(_promptSuggestion)}\n\n');
        res.write(
          'event: finished\ndata: {"user":{"id":"u","role":"user","content":"why?"},'
          '"assistant":{"id":"a","role":"assistant","content":"Because"},"suggestions":[]}\n\n',
        );
      case ('GET', '$_out/suggestions') || ('POST', '$_out/suggestions/generate'):
        res.write(
          jsonEncode({
            'suggestions': [_promptSuggestion],
          }),
        );
      case ('POST', '$_out/suggestions/s2/apply'):
        res.write(
          jsonEncode({
            'suggestion': {..._promptSuggestion, 'status': 'applied'},
            'promptName': 'refined-1',
          }),
        );
      case ('POST', '$_out/suggestions/s3/apply'):
        res.statusCode = 409;
        res.write(jsonEncode({'error': 'the prompt changed after this suggestion was made'}));
      case ('POST', '$_out/suggestions/s2/dismiss'):
        res.write(
          jsonEncode({
            'suggestion': {..._promptSuggestion, 'status': 'dismissed'},
          }),
        );
      case ('POST', '$_out/versions'):
        res.statusCode = 201;
        res.write(
          jsonEncode({
            'id': 'v2id',
            'title': 'Digest v2',
            'workType': 'summarize',
            'notebookId': 'board',
            'parentId': '20261121T100000-abcdef',
            'version': 2,
          }),
        );
      default:
        res.statusCode = 404;
        res.write(jsonEncode({'error': 'not found'}));
    }
    await res.close();
  });
  return (server, calls);
}

void main() {
  late HttpServer server;
  late List<String> calls;
  late HttpCoreClient client;
  const nb = 'board', out = '20261121T100000-abcdef';

  setUp(() async {
    (server, calls) = await _fakeThreads();
    client = HttpCoreClient(
      Endpoint(network: 'tcp', address: '127.0.0.1:${server.port}', token: _token),
      client: realHttpClient(),
    );
  });
  tearDown(() async {
    client.close();
    await server.close(force: true);
  });

  test('maps outputs with version and parent', () {
    final o = outputFromJson({'id': 'x', 'parentId': 'p', 'version': 3});
    expect((o.parentId, o.version), ('p', 3));
    expect(outputFromJson({'id': 'y'}).version, 1);
  });

  test('maps suggestions with their target', () {
    final s = suggestionFromJson(_promptSuggestion);
    expect((s.id, s.kind, s.status, s.isOpen), ('s2', 'prompt', 'open', true));
    expect(s.target!.promptName, 'weekly');
    expect(s.target!.reach, 'Updates prompt weekly, used by 2 schedules');
    expect(suggestionFromJson({'kind': 'memory', 'text': 't'}).target, isNull);
    expect(const PromptTarget(kind: 'schedule', scheduleId: 'c', current: '').reach, 'Updates schedule c');
    expect(const PromptTarget(kind: 'new', current: '').reach, 'Saves a new prompt');
  });

  test('reads and patches the thread; notes keep their role', () async {
    final sess = await client.getThread(nb, out);
    expect(sess.messages.map((m) => m.role), ['user', 'note']);
    await client.setThreadSettings(nb, out, thinkingEffort: 'high');
    expect(calls.last, contains('"thinkingEffort":"high"'));
  });

  test('streams delta, suggestion and finished', () async {
    final events = await client.sendThread(nb, out, 'why?').toList();
    expect(events[0], isA<ThreadDelta>());
    expect((events[1] as ThreadSuggestion).suggestion.id, 's2');
    expect((events[2] as ThreadFinished).assistant.content, 'Because');
  });

  test('lists, generates, applies, dismisses and versions', () async {
    expect(await client.listSuggestions(nb, out), hasLength(1));
    expect(await client.generateSuggestions(nb, out), hasLength(1));
    final applied = await client.applySuggestion(nb, out, 's2');
    expect((applied.suggestion.status, applied.promptName), ('applied', 'refined-1'));
    expect((await client.dismissSuggestion(nb, out, 's2')).status, 'dismissed');
    final v = await client.newVersion(nb, out, fromMessageId: 'a');
    expect((v.id, v.version, v.parentId), ('v2id', 2, out));
    expect(calls.last, contains('"fromMessageId":"a"'));
  });

  test('maps 409 to CoreException with the server message', () async {
    await expectLater(
      client.applySuggestion(nb, out, 's3'),
      throwsA(isA<CoreException>().having((e) => e.message, 'message', contains('prompt changed'))),
    );
  });

  test('demo core: thread, suggestions and a new version', () async {
    final core = FakeCoreClient(tick: Duration.zero);
    final outs = await core.listOutputs(notebookId: 'nb-1');
    final first = outs.firstWhere((o) => o.version == 1);
    final thread = await core.getThread('nb-1', first.id);
    expect(thread.messages, isNotEmpty);
    final open = (await core.listSuggestions('nb-1', first.id)).where((s) => s.isOpen).toList();
    expect(open.single.target?.kind, 'saved');
    final applied = await core.applySuggestion('nb-1', first.id, open.single.id);
    expect(applied.suggestion.status, 'applied');
    final events = await core.sendThread('nb-1', first.id, 'why?').toList();
    expect(events.last, isA<ThreadFinished>());
    final v = await core.newVersion('nb-1', first.id);
    expect(v.parentId, first.id);
    expect((await core.listOutputs(notebookId: 'nb-1')).map((o) => o.id), contains(v.id));
    await expectLater(core.applySuggestion('nb-1', first.id, open.single.id), throwsA(isA<CoreException>()));
  });
}
