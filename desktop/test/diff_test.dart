import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:tessivo_desktop/src/core/http_core_client.dart';

import 'notebook_page_test.dart' show pumpPage;
import 'support/real_http.dart';

void main() {
  testWidgets('a revision compares with the version it revised', (tester) async {
    final state = await pumpPage(tester);
    expect(find.byKey(const Key('compare')), findsNothing); // o1 is an original
    await tester.tap(find.byKey(const Key('new-version')));
    await tester.pumpAndSettle();
    expect(state.selectedOutputId, 'o1-v2');
    await tester.tap(find.byKey(const Key('compare')));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('diff-view')), findsOneWidget);
    expect(find.textContaining('added'), findsWidgets);
    expect(find.textContaining('+ '), findsWidgets);
  });

  test('http client fetches a diff', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    server.listen((req) async {
      expect(req.uri.queryParameters['against'], 'o1');
      req.response.write(
        jsonEncode({
          'from': 'o1',
          'to': 'o2',
          'lines': [
            {'op': 32, 'text': 'same'},
            {'op': 43, 'text': 'new'},
            {'op': 45, 'text': 'old'},
          ],
          'counts': {'added': 1, 'removed': 1},
        }),
      );
      await req.response.close();
    });
    final client = HttpCoreClient(
      Endpoint(network: 'tcp', address: '127.0.0.1:${server.port}', token: 't'),
      client: realHttpClient(),
    );
    final d = await client.diffOutput('ops', 'o2', against: 'o1');
    expect(d.lines.map((l) => l.op).join(), ' +-');
    expect((d.added, d.removed), (1, 1));
    client.close();
    await server.close(force: true);
  });
}
