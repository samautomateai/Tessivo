import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:tessivo_desktop/src/core/http_core_client.dart';

import 'support/fake_core/fake_core.dart';
import 'support/real_http.dart';

const _nb = {
  'id': 'board',
  'name': 'Board',
  'description': 'Q3',
  'notes': 1,
  'sources': [
    {'id': 's1', 'label': 'minutes.md', 'kind': 'file', 'level': 'summary', 'summary': 'ready', 'tokens': 1200},
    {'id': 's2', 'label': 'payments', 'kind': 'folder', 'level': 'full', 'summary': 'pending'},
  ],
};

Future<(HttpServer, List<String>)> _fake() async {
  final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
  final calls = <String>[];
  server.listen((req) async {
    final res = req.response;
    calls.add('${req.method} ${req.uri.path} ${await utf8.decoder.bind(req).join()}');
    const c = '/v1/research/board/chats/aaaaaaaaaaaaaaaa';
    switch ((req.method, req.uri.path)) {
      case ('GET', '/v1/research'):
        res.write(
          jsonEncode({
            'notebooks': [_nb],
          }),
        );
      case ('POST', '/v1/research'):
        res.statusCode = 201;
        res.write(jsonEncode(_nb));
      case ('GET', '/v1/research/board') ||
          ('PATCH', '/v1/research/board/sources/s1') ||
          ('POST', '/v1/research/board/sources'):
        res.write(jsonEncode(_nb));
      case ('GET', '/v1/research/board/context'):
        res.write(
          jsonEncode({
            'sources': 2,
            'summaries': 1,
            'notes': 1,
            'tokens': 14000,
            'cut': ['payments'],
          }),
        );
      case ('GET', '/v1/research/board/sources/s1'):
        res.write(jsonEncode({'text': 'full text', 'summary': 'short'}));
      case ('POST', '/v1/research/board/chats'):
        res.statusCode = 201;
        res.write(jsonEncode({'id': 'aaaaaaaaaaaaaaaa', 'title': 'Plans', 'messages': <Object>[]}));
      case ('GET', c):
        res.write(
          jsonEncode({
            'session': {
              'id': 'aaaaaaaaaaaaaaaa',
              'title': 'Plans',
              'messages': [
                {'id': 'm2', 'role': 'assistant', 'content': 'March [1].'},
              ],
            },
            'citations': {
              'm2': [
                {'n': 1, 'kind': 'source', 'id': 's1', 'label': 'minutes.md', 'level': 'summary'},
              ],
            },
          }),
        );
      case ('POST', '$c/messages'):
        res.headers.contentType = ContentType('text', 'event-stream');
        res.write('event: delta\ndata: {"text":"March [1]."}\n\n');
        res.write(
          'event: finished\ndata: {"user":{"id":"u","role":"user","content":"When?"},'
          '"assistant":{"id":"a","role":"assistant","content":"March [1]."},'
          '"citations":[{"n":1,"kind":"source","id":"s1","label":"minutes.md","level":"summary"}],'
          '"skipped":["payments"]}\n\n',
        );
      case ('POST', '/v1/research/board/chats/bbbbbbbbbbbbbbbb/messages'):
        res.headers.contentType = ContentType('text', 'event-stream');
        res.write('event: error\ndata: {"status":409,"error":"a reply is already streaming"}\n\n');
      case ('POST', '$c/messages/a/note'):
        res.statusCode = 201;
        res.write(jsonEncode({'id': 'n9', 'title': 'When?', 'text': 'March', 'origin': 'ai', 'include': true}));
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
  setUp(() async {
    (server, calls) = await _fake();
    client = HttpCoreClient(
      Endpoint(network: 'tcp', address: '127.0.0.1:${server.port}', token: 't'),
      client: realHttpClient(),
    );
  });
  tearDown(() async {
    client.close();
    await server.close(force: true);
  });

  test('maps notebooks, sources and totals', () async {
    final list = await client.listResearch();
    final s1 = list.single.sources.first;
    expect((s1.label, s1.level, s1.summary, s1.tokens), ('minutes.md', 'summary', 'ready', 1200));
    expect(list.single.sources[1].hasSummary, isFalse);
    final ctx = await client.researchContext('board');
    expect((ctx.sources, ctx.summaries, ctx.tokens), (2, 1, 14000));
    expect(ctx.cut, ['payments']);
    final src = await client.researchSource('board', 's1');
    expect((src.text, src.summary), ('full text', 'short'));
  });

  test('adds sources, sets levels and creates notebooks', () async {
    await client.createResearch('Board', description: 'Q3');
    expect(calls.last, contains('"description":"Q3"'));
    await client.addResearchSource('board', path: '/tmp/x.md');
    expect(calls.last, contains('"path":"/tmp/x.md"'));
    await client.setSourceLevel('board', 's1', 'full');
    expect(calls.last, contains('"level":"full"'));
  });

  test('streams an answer with citations and saves it as a note', () async {
    final chat = await client.createResearchChat('board', title: 'Plans');
    final events = await client.sendResearch('board', chat.id, 'When?').toList();
    final done = events.last as ResearchFinished;
    expect(done.citations.single.label, 'minutes.md');
    expect(done.skipped, ['payments']);
    final stored = await client.getResearchChat('board', chat.id);
    expect(stored.citations['m2']!.single.n, 1);
    final note = await client.noteFromMessage('board', chat.id, 'a');
    expect((note.origin, note.include), ('ai', true));
  });

  test('a busy conversation is a CoreException', () async {
    await expectLater(
      client.sendResearch('board', 'bbbbbbbbbbbbbbbb', 'x').toList(),
      throwsA(isA<CoreException>().having((e) => e.status, 'status', 409)),
    );
  });

  test('demo core: add a source, its summary is ready, an answer cites it', () async {
    final core = FakeCoreClient(tick: Duration.zero);
    final nb = (await core.listResearch()).first;
    final added = await core.addResearchSource(nb.id, path: '/data/report.md');
    expect(added.sources.last.summary, 'ready');
    final chat = await core.createResearchChat(nb.id);
    final events = await core.sendResearch(nb.id, chat.id, 'What changed?').toList();
    expect((events.last as ResearchFinished).citations, isNotEmpty);
    final updated = await core.setSourceLevel(nb.id, added.sources.last.id, 'off');
    expect(updated.sources.last.level, 'off');
    final ctx = await core.researchContext(nb.id);
    expect(ctx.sources, updated.sources.where((s) => s.level != 'off').length);
  });
}
