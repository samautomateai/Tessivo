import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:tessivo_desktop/src/core/http_core_client.dart';

import 'app_test.dart' show pumpApp;
import 'support/real_http.dart';

Future<void> _menu(WidgetTester tester, String item) async {
  await tester.tap(find.byKey(const Key('notebook-menu')));
  await tester.pumpAndSettle();
  await tester.tap(find.byKey(Key('notebook-$item')));
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('rename, archive and restore a notebook', (tester) async {
    final state = await pumpApp(tester);
    await _menu(tester, 'rename');
    await tester.enterText(find.byType(TextField).last, 'Risk review v2');
    await tester.tap(find.text('OK'));
    await tester.pumpAndSettle();
    expect(state.selected?.name, 'Risk review v2');
    expect(find.text('Risk review v2'), findsWidgets);

    await _menu(tester, 'archive');
    expect(state.notebooks.firstWhere((n) => n.id == 'nb-1').archived, isTrue);
    expect(find.byKey(const Key('archived-toggle')), findsOneWidget);
    expect(find.byKey(const Key('nav-nb-1')), findsNothing);
    await tester.tap(find.byKey(const Key('archived-toggle')));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('nav-nb-1')), findsOneWidget);

    await tester.tap(find.byKey(const Key('nav-nb-1')));
    await tester.pumpAndSettle();
    await _menu(tester, 'restore');
    expect(state.notebooks.firstWhere((n) => n.id == 'nb-1').archived, isFalse);
  });

  testWidgets('delete asks first, then removes the notebook', (tester) async {
    final state = await pumpApp(tester);
    await _menu(tester, 'delete');
    expect(find.textContaining('cannot be undone'), findsOneWidget);
    await tester.tap(find.text('Cancel'));
    await tester.pumpAndSettle();
    expect(state.notebooks.any((n) => n.id == 'nb-1'), isTrue);
    await _menu(tester, 'delete');
    await tester.tap(find.byKey(const Key('confirm-delete')));
    await tester.pumpAndSettle();
    expect(state.notebooks.any((n) => n.id == 'nb-1'), isFalse);
    expect(state.selectedId, isNot('nb-1'));
  });

  test('http client patches and deletes notebooks', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    final calls = <String>[];
    server.listen((req) async {
      calls.add('${req.method} ${req.uri.path} ${await utf8.decoder.bind(req).join()}');
      if (req.method == 'DELETE') {
        req.response.statusCode = 204;
      } else {
        req.response.write(jsonEncode({'id': 'ops', 'name': 'Weekly', 'workType': 'report', 'archived': true}));
      }
      await req.response.close();
    });
    final client = HttpCoreClient(
      Endpoint(network: 'tcp', address: '127.0.0.1:${server.port}', token: 't'),
      client: realHttpClient(),
    );
    final nb = await client.updateNotebook('ops', name: 'Weekly', archived: true);
    expect((nb.name, nb.archived), ('Weekly', true));
    expect(calls.last, 'PATCH /v1/notebooks/ops {"name":"Weekly","archived":true}');
    await client.deleteNotebook('ops');
    expect(calls.last, startsWith('DELETE /v1/notebooks/ops'));
    client.close();
    await server.close(force: true);
  });
}
