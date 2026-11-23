import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:tessivo_desktop/src/app_state.dart';
import 'package:tessivo_desktop/src/core/http_core_client.dart';

import 'app_test.dart' show pumpApp;
import 'support/real_http.dart';

void main() {
  testWidgets('search finds a notebook and a research notebook and opens them', (tester) async {
    final state = await pumpApp(tester);
    await tester.enterText(find.byKey(const Key('sidebar-search')), 'weekly');
    await tester.testTextInput.receiveAction(TextInputAction.search);
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('search-results')), findsOneWidget);
    await tester.tap(find.textContaining('Weekly ops report').last);
    await tester.pumpAndSettle();
    expect((state.section, state.selected?.name), (Section.notebooks, 'Weekly ops report'));

    await tester.enterText(find.byKey(const Key('sidebar-search')), 'board');
    await tester.testTextInput.receiveAction(TextInputAction.search);
    await tester.pumpAndSettle();
    await tester.tap(find.text('Board papers').last);
    await tester.pumpAndSettle();
    expect(state.section, Section.research);

    await tester.enterText(find.byKey(const Key('sidebar-search')), 'zzz nothing');
    await tester.testTextInput.receiveAction(TextInputAction.search);
    await tester.pumpAndSettle();
    expect(find.textContaining('Nothing found'), findsOneWidget);
  });

  test('http client searches', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    server.listen((req) async {
      expect(req.uri.queryParameters['q'], 'risk plan');
      req.response.write(
        jsonEncode({
          'results': [
            {'kind': 'output', 'notebookId': 'ops', 'id': 'o1', 'title': 'Risk', 'snippet': '…risk plan…'},
          ],
        }),
      );
      await req.response.close();
    });
    final client = HttpCoreClient(
      Endpoint(network: 'tcp', address: '127.0.0.1:${server.port}', token: 't'),
      client: realHttpClient(),
    );
    final hits = await client.search('risk plan');
    expect((hits.single.kind, hits.single.id, hits.single.snippet), ('output', 'o1', '…risk plan…'));
    client.close();
    await server.close(force: true);
  });
}
