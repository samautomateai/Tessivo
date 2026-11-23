import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:tessivo_desktop/src/core/http_core_client.dart';

import 'app_test.dart' show pumpApp;
import 'support/real_http.dart';

void main() {
  testWidgets('add a web page and pasted text as sources', (tester) async {
    final state = await pumpApp(tester);
    await tester.tap(find.byKey(const Key('context-add-more')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('context-add-url')));
    await tester.pumpAndSettle();
    await tester.enterText(find.byType(TextField).last, 'https://status.example.com/q3');
    await tester.tap(find.text('OK'));
    await tester.pumpAndSettle();
    expect(find.text('status.example.com/q3'), findsOneWidget);
    expect(find.text('Web page · read at run time'), findsOneWidget);

    await tester.tap(find.byKey(const Key('context-add-more')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('context-add-text')));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const Key('paste-label')), 'Meeting notes');
    await tester.enterText(find.byKey(const Key('paste-text')), 'Decided: ship in March.');
    await tester.tap(find.byKey(const Key('paste-save')));
    await tester.pumpAndSettle();
    expect(state.selected!.context.map((c) => c.label), contains('Meeting notes'));
  });

  testWidgets('add git changes as a source', (tester) async {
    final state = await pumpApp(tester);
    await tester.tap(find.byKey(const Key('context-add-more')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('context-add-git')));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const Key('git-repo')), '/src/payments');
    await tester.enterText(find.byKey(const Key('git-base')), 'develop');
    await tester.tap(find.byKey(const Key('git-save')));
    await tester.pumpAndSettle();
    final item = state.selected!.context.last;
    expect((item.label, item.kind), ('payments changes since develop', 'Git changes'));
    expect(find.text('Git changes · read at run time'), findsOneWidget);
  });

  test('http client sends url and text sources', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    final bodies = <String>[];
    server.listen((req) async {
      bodies.add(await utf8.decoder.bind(req).join());
      req.response.write(jsonEncode({'id': 'ops', 'name': 'Ops', 'workType': 'report', 'context': <Object>[]}));
      await req.response.close();
    });
    final client = HttpCoreClient(
      Endpoint(network: 'tcp', address: '127.0.0.1:${server.port}', token: 't'),
      client: realHttpClient(),
    );
    await client.addUrlContext('ops', 'https://x.example/a', label: 'A');
    await client.addTextContext('ops', 'hello', label: 'Notes');
    await client.addGitDiffContext('ops', '/src/a', 'main');
    expect(bodies, [
      '{"url":"https://x.example/a","label":"A"}',
      '{"text":"hello","label":"Notes"}',
      '{"git":{"repo":"/src/a","base":"main"},"label":""}',
    ]);
    client.close();
    await server.close(force: true);
  });
}
