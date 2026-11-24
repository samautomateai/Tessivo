import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:tessivo_desktop/main.dart';
import 'package:tessivo_desktop/src/app_state.dart';
import 'package:tessivo_desktop/src/core/http_core_client.dart';

import 'support/fake_core/fake_core.dart';
import 'support/real_http.dart';

const _server = {
  'name': 'fake',
  'command': 'npx',
  'args': ['-y', 'srv'],
  'env': {'GH_TOKEN': r'${secret:GH}'},
};

/// Fake of the core's MCP endpoints; records method, path and body.
Future<(HttpServer, List<String>)> _fakeMcp() async {
  final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
  final calls = <String>[];
  server.listen((req) async {
    final res = req.response;
    calls.add('${req.method} ${req.uri.path} ${await utf8.decoder.bind(req).join()}');
    final nb = {
      'id': 'ops',
      'name': 'Ops',
      'workType': 'summarize',
      'mcpServers': ['fake'],
      'context': [
        {'id': 'c1', 'label': 'file:///n.md', 'kind': 'mcp-resource', 'server': 'fake', 'enabled': true},
      ],
    };
    switch ((req.method, req.uri.path)) {
      case ('GET', '/v1/mcp/servers'):
        res.write(
          jsonEncode({
            'servers': [_server],
          }),
        );
      case ('POST', '/v1/mcp/servers'):
        res.statusCode = 201;
        res.write(jsonEncode(_server));
      case ('DELETE', '/v1/mcp/servers/fake'):
        res.statusCode = 409;
        res.write(jsonEncode({'error': 'mcp server "fake" is attached to Ops; detach it first'}));
      case ('POST', '/v1/mcp/servers/fake/test'):
        res.write(
          jsonEncode({
            'tools': [
              {'name': 'echo', 'description': 'Echoes'},
            ],
            'resources': [
              {'uri': 'file:///n.md', 'name': 'notes'},
            ],
            'prompts': <Object>[],
          }),
        );
      case ('PUT', '/v1/notebooks/ops/mcp') || ('POST', '/v1/notebooks/ops/context'):
        res.write(jsonEncode(nb));
      default:
        res.statusCode = 404;
        res.write(jsonEncode({'error': 'not found'}));
    }
    await res.close();
  });
  return (server, calls);
}

Future<AppState> _pump(WidgetTester tester) async {
  tester.view.physicalSize = const Size(1600, 1100);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  final state = AppState(FakeCoreClient(tick: Duration.zero));
  await tester.pumpWidget(TessivoApp(state: state));
  await state.load();
  await tester.pumpAndSettle();
  return state;
}

void main() {
  group('http', () {
    late HttpServer server;
    late List<String> calls;
    late HttpCoreClient client;
    setUp(() async {
      (server, calls) = await _fakeMcp();
      client = HttpCoreClient(
        Endpoint(network: 'tcp', address: '127.0.0.1:${server.port}', token: 't'),
        client: realHttpClient(),
      );
    });
    tearDown(() async {
      client.close();
      await server.close(force: true);
    });

    test('lists, adds, tests and maps a 409 on remove', () async {
      final list = await client.listMcpServers();
      expect(
        (list.single.name, list.single.transport, list.single.env['GH_TOKEN']),
        ('fake', 'stdio', r'${secret:GH}'),
      );
      await client.addMcpServer(const McpServer(name: 'fake', command: 'npx', args: ['-y', 'srv']));
      expect(calls.last, contains('"command":"npx"'));
      final t = await client.testMcpServer('fake');
      expect((t.tools.single.name, t.resources.single.uri), ('echo', 'file:///n.md'));
      expect(t.prompts, isEmpty);
      await expectLater(
        client.removeMcpServer('fake'),
        throwsA(isA<CoreException>().having((e) => e.message, 'message', contains('detach it first'))),
      );
    });

    test('attaches servers and adds a resource to a notebook', () async {
      final nb = await client.setNotebookMcp('ops', ['fake']);
      expect(nb.mcpServers, ['fake']);
      expect(calls.last, contains('"servers":["fake"]'));
      final withRes = await client.addMcpContext('ops', 'fake', 'file:///n.md');
      expect(withRes.context.single.isMcp, isTrue);
      expect(withRes.context.single.kind, 'MCP · fake');
      expect(calls.last, contains('"uri":"file:///n.md"'));
    });
  });

  testWidgets('settings adds, tests and removes an MCP server', (tester) async {
    await _pump(tester);
    await tester.tap(find.text('Settings'));
    await tester.pumpAndSettle();
    await _toMcp(tester);
    await tester.tap(find.byKey(const Key('mcp-add')));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const Key('mcp-name')), 'docs');
    await tester.enterText(find.byKey(const Key('mcp-url')), 'https://mcp.example.com/mcp');
    await tester.enterText(find.byKey(const Key('mcp-headers')), r'Authorization=Bearer ${secret:DOCS}');
    await tester.tap(find.byKey(const Key('mcp-save')));
    await tester.pumpAndSettle();
    expect(find.text('docs'), findsOneWidget);
    await tester.tap(find.byKey(const Key('mcp-test-docs')));
    await tester.pumpAndSettle();
    expect(find.textContaining('search'), findsWidgets);
    await tester.tap(find.text('Close'));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('mcp-remove-docs')));
    await tester.pumpAndSettle();
    expect(find.text('docs'), findsNothing);
  });

  testWidgets('settings refuses a server without a name', (tester) async {
    await _pump(tester);
    await tester.tap(find.text('Settings'));
    await tester.pumpAndSettle();
    await _toMcp(tester);
    await tester.tap(find.byKey(const Key('mcp-add')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('mcp-save')));
    await tester.pumpAndSettle();
    expect(find.textContaining('Give a name'), findsOneWidget);
  });

  testWidgets('a notebook attaches a server and adds a resource', (tester) async {
    final state = await _pump(tester);
    await tester.tap(find.byKey(const Key('nb-mcp-github')));
    await tester.pumpAndSettle();
    expect(state.selected!.mcpServers, ['github']);
    await tester.tap(find.byKey(const Key('context-add-mcp')));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const Key('mcp-resource-uri')), 'repo://payments/README.md');
    await tester.tap(find.byKey(const Key('mcp-resource-add')));
    await tester.pumpAndSettle();
    expect(find.text('repo://payments/README.md'), findsOneWidget);
    expect(find.textContaining('read at run time'), findsOneWidget);
  });
}

/// Scrolls Settings until the MCP servers section is on screen.
Future<void> _toMcp(WidgetTester tester) => tester.scrollUntilVisible(
  find.byKey(const Key('mcp-add')),
  200,
  scrollable: find.descendant(of: find.byKey(const Key('settings-list')), matching: find.byType(Scrollable)).first,
);
