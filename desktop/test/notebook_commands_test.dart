import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:tessivo_desktop/src/core/command_rule.dart';
import 'package:tessivo_desktop/src/core/http_core_client.dart';

import 'app_test.dart' show pumpApp;
import 'support/real_http.dart';

void main() {
  testWidgets('allow and remove agent commands on the notebook page', (tester) async {
    final state = await pumpApp(tester);
    await tester.ensureVisible(find.byKey(const Key('command-add')));
    await tester.tap(find.byKey(const Key('command-add')));
    await tester.pumpAndSettle();
    await tester.enterText(find.byType(TextField).last, '  go   test *  ');
    await tester.tap(find.text('OK').last);
    await tester.pumpAndSettle();
    expect([for (final c in state.selected!.commands) '$c'], ['go test *']);
    expect(find.text('go test *'), findsOneWidget);
    await tester.tap(find.byKey(const Key('command-remove-0')));
    await tester.pumpAndSettle();
    expect(state.selected!.commands, isEmpty);
  });

  test('rules parse, print and round-trip as JSON', () {
    expect(CommandRule.parse('   '), isNull);
    final r = CommandRule.parse('make  lint')!;
    expect(r.program, 'make');
    expect(r.args, ['lint']);
    expect(r.toJson(), {
      'program': 'make',
      'args': ['lint'],
    });
    expect(const CommandRule(program: 'ls').toJson(), {'program': 'ls'});
    final back = commandRulesFromJson(jsonDecode(jsonEncode([r.toJson(), 'junk'])));
    expect([for (final b in back) '$b'], ['make lint']);
  });

  test('http client puts the allow-list', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    String? got;
    server.listen((req) async {
      got = '${req.method} ${req.uri.path} ${await utf8.decoder.bind(req).join()}';
      req.response.write(
        jsonEncode({
          'id': 'ops',
          'name': 'Ops',
          'workType': 'analyze',
          'context': <Object>[],
          'commands': [
            {
              'program': 'go',
              'args': ['test', '*'],
            },
          ],
        }),
      );
      await req.response.close();
    });
    final client = HttpCoreClient(
      Endpoint(network: 'tcp', address: '127.0.0.1:${server.port}', token: 't'),
      client: realHttpClient(),
    );
    final nb = await client.setNotebookCommands('ops', [
      const CommandRule(program: 'go', args: ['test', '*']),
    ]);
    client.close();
    await server.close(force: true);
    expect(got, 'PUT /v1/notebooks/ops/agent/commands {"commands":[{"program":"go","args":["test","*"]}]}');
    expect('${nb.commands.single}', 'go test *');
  });
}
