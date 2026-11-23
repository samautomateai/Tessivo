import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:tessivo_desktop/src/core/core_client.dart';
import 'package:tessivo_desktop/src/core/http_core_client.dart';

import 'chat_test.dart' show pumpChat;
import 'support/real_http.dart';

void main() {
  testWidgets('a chat with an MCP server asks before a tool runs', (tester) async {
    final state = await pumpChat(tester);
    await tester.tap(find.byKey(const Key('chat-c-1')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('chat-tools')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('chat-tool-github')));
    await tester.tap(find.byKey(const Key('chat-tools-save')));
    await tester.pumpAndSettle();
    expect(state.chat.open!.mcpServers, ['github']);

    await tester.enterText(find.byKey(const Key('chat-input')), 'Open issues?');
    await tester.tap(find.byKey(const Key('chat-send')));
    await tester.pump();
    await tester.pump();
    expect(find.byKey(const Key('approval-card')), findsOneWidget);
    await tester.tap(find.byKey(const Key('tool-allow')));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('approval-card')), findsNothing);
    expect(find.textContaining('mcp__github__list_issues'), findsWidgets);
  });

  test('http client streams tool events and answers approvals', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    final calls = <String>[];
    server.listen((req) async {
      calls.add('${req.method} ${req.uri.path} ${await utf8.decoder.bind(req).join()}');
      if (req.uri.path.endsWith('/messages')) {
        req.response.headers.contentType = ContentType('text', 'event-stream');
        req.response.write('event: approval\ndata: {"id":"c1","tool":"mcp__gh__x","args":"{}"}\n\n');
        req.response.write('event: tool\ndata: {"kind":"result","tool":"mcp__gh__x","summary":"3 issues"}\n\n');
      } else if (req.method == 'PATCH') {
        req.response.write(
          jsonEncode({
            'id': 'aaaaaaaaaaaaaaaa',
            'title': 't',
            'mcpServers': ['gh'],
          }),
        );
      } else {
        req.response.statusCode = 204;
      }
      await req.response.close();
    });
    final client = HttpCoreClient(
      Endpoint(network: 'tcp', address: '127.0.0.1:${server.port}', token: 't'),
      client: realHttpClient(),
    );
    const id = 'aaaaaaaaaaaaaaaa';
    final s = await client.updateChat(id, mcpServers: ['gh']);
    expect(s.mcpServers, ['gh']);
    expect(calls.last, contains('"mcpServers":["gh"]'));
    final events = await client.sendChat(id, 'q').toList();
    expect((events[0] as ChatApprovalNeeded).tool, 'mcp__gh__x');
    expect((events[1] as ChatToolActivity).summary, '3 issues');
    await client.answerChatTool(id, 'c1', allow: true, remember: true);
    expect(calls.last, 'POST /v1/chats/$id/approvals/c1 {"allow":true,"remember":true}');
    client.close();
    await server.close(force: true);
  });
}
