import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:tessivo_desktop/src/core/core_client.dart';
import 'package:tessivo_desktop/src/core/http_core_client.dart';
import 'package:tessivo_desktop/src/core/models.dart';

const token = 'secret-token';

const sessionJson = {
  'id': 'aaaaaaaaaaaaaaaa',
  'title': 'Keys',
  'pinned': true,
  'model': 'smart',
  'thinkingEffort': 'low',
  'messages': [
    {
      'id': 'm1',
      'role': 'user',
      'content': 'hi',
      'attachments': [
        {'kind': 'path', 'ref': '/r/runbook.md', 'tokens': 1500},
      ],
    },
    {'id': 'm2', 'role': 'assistant', 'content': 'part', 'stopped': true, 'error': 'model error: rate_limit'},
  ],
};

/// Fake of the core's chat endpoints; records request bodies.
Future<(HttpServer, List<Map<String, Object?>>)> fakeChat() async {
  final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
  final bodies = <Map<String, Object?>>[];
  server.listen((req) async {
    final res = req.response;
    final text = await utf8.decoder.bind(req).join();
    if (text.isNotEmpty) bodies.add(jsonDecode(text) as Map<String, Object?>);
    final p = req.uri.path;
    const c = '/v1/chats/aaaaaaaaaaaaaaaa';
    switch ((req.method, p)) {
      case ('GET', '/v1/chats'):
        res.write(
          jsonEncode({
            'sessions': [
              {'id': 'aaaaaaaaaaaaaaaa', 'title': 'Keys', 'messageCount': 2, 'preview': 'part', 'q': req.uri.query},
            ],
          }),
        );
      case ('POST', '/v1/chats'):
        res.statusCode = 201;
        res.write(jsonEncode({...sessionJson, 'messages': <Object>[]}));
      case ('GET', c) || ('PATCH', c):
        res.write(jsonEncode(sessionJson));
      case ('DELETE', c):
        res.statusCode = 204;
      case ('POST', '$c/messages'):
        res.headers.contentType = ContentType('text', 'event-stream');
        res.write('event: delta\ndata: {"text":"Hel"}\n\n');
        res.write('event: delta\ndata: garbage\n\n');
        res.write('event: delta\ndata: {"text":"lo"}\n\n');
        res.write(
          'event: finished\ndata: {"user":{"id":"u","role":"user","content":"hi"},'
          '"assistant":{"id":"a","role":"assistant","content":"Hello","model":"litellm/m"}}\n\n',
        );
      case ('POST', '/v1/chats/bbbbbbbbbbbbbbbb/messages'):
        res.headers.contentType = ContentType('text', 'event-stream');
        res.write('event: error\ndata: {"status":502,"error":"model error: server_error"}\n\n');
      case ('GET' || 'PUT', '/v1/chat-memory'):
        res.write(
          jsonEncode({
            'memory': ['I run ops', 7],
          }),
        );
      case ('GET', '$c/repeatable'):
        res.write(
          jsonEncode({
            'name': 'Keys',
            'workType': 'analyze',
            'prompt': 'hi',
            'templateId': 'change-risk-review',
            'templateName': 'Change risk review',
            'context': ['/r/runbook.md'],
            'skipped': ['Digest: outputs'],
          }),
        );
      case ('POST', '$c/notebook'):
        res.statusCode = 201;
        res.write(jsonEncode({'id': 'keys', 'name': 'Keys', 'workType': 'analyze', 'context': <Object>[]}));
      case ('POST', '$c/messages/m2/output'):
        res.statusCode = 201;
        res.write(jsonEncode({'notebookId': 'chat-report', 'outputId': 'o1', 'title': 'Digest'}));
      default:
        res.statusCode = 404;
        res.write(jsonEncode({'error': 'chat session not found'}));
    }
    await res.close();
  });
  return (server, bodies);
}

void main() {
  late HttpServer server;
  late List<Map<String, Object?>> bodies;
  late HttpCoreClient client;
  const id = 'aaaaaaaaaaaaaaaa';

  setUp(() async {
    (server, bodies) = await fakeChat();
    client = HttpCoreClient(Endpoint(network: 'tcp', address: '127.0.0.1:${server.port}', token: token));
  });
  tearDown(() async {
    client.close();
    await server.close(force: true);
  });

  test('sessions: list, create, get, update, delete', () async {
    final list = await client.listChats(query: 'keys', archived: true);
    expect(list.single.messageCount, 2);
    expect((await client.createChat(title: 'x')).messages, isEmpty);
    expect(bodies.last, {'title': 'x', 'model': '', 'thinkingEffort': ''});

    final s = await client.getChat(id);
    expect(s.pinned, isTrue);
    expect(s.messages.first.attachments.single.display, 'runbook.md');
    expect(s.messages.last.stopped, isTrue);
    expect(s.messages.last.error, contains('rate_limit'));

    await client.updateChat(id, pinned: false, title: 'New');
    expect(bodies.last, {'title': 'New', 'pinned': false});
    await client.deleteChat(id);
    expect(() => client.deleteChat('bbbbbbbbbbbbbbbb'), throwsA(isA<CoreException>()));
  });

  test('send streams deltas, skips garbage and ends with the stored turns', () async {
    const att = ChatAttachment(kind: 'path', ref: '/r/a.md');
    final events = await client.sendChat(id, 'hi', attachments: const [att]).toList();
    expect(events.whereType<ChatDelta>().map((e) => e.text).join(), 'Hello');
    final done = events.last as ChatFinished;
    expect(done.assistant.model, 'litellm/m');
    expect(bodies.last['attachments'], [
      {'kind': 'path', 'ref': '/r/a.md'},
    ]);
  });

  test('send reports stream errors, HTTP errors and bad input', () async {
    expect(client.sendChat('bbbbbbbbbbbbbbbb', 'hi').toList(), throwsA(isA<CoreException>()));
    expect(client.sendChat('cccccccccccccccc', 'hi').toList(), throwsA(isA<CoreException>()));
    expect(client.sendChat(id, '   ').toList(), throwsFormatException);
  });

  test('memory, repeatable plan, make notebook and save reply', () async {
    expect(await client.chatMemory(), ['I run ops']);
    expect(await client.setChatMemory(['I run ops']), ['I run ops']);
    expect(() => client.setChatMemory(List.filled(maxMemoryItems + 1, 'x')), throwsFormatException);

    final plan = await client.planRepeatable(id);
    expect(plan.workType, WorkTypeId.analyze);
    expect(plan.templateName, 'Change risk review');
    expect(plan.skipped.single, startsWith('Digest'));

    final nb = await client.makeRepeatable(id, name: 'Keys', workType: WorkTypeId.analyze, templateId: 'none');
    expect(nb.id, 'keys');
    expect(bodies.last, {'name': 'Keys', 'workType': 'analyze', 'templateId': 'none'});
    await client.makeRepeatable(id);
    expect(bodies.last, isEmpty);

    final out = await client.saveReply(id, 'm2', WorkTypeId.report);
    expect(out.notebookId, 'chat-report');
    expect(bodies.last, {'workType': 'report'});
  });
}
