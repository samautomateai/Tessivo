import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:tessivo_desktop/src/core/http_core_client.dart';

import 'app_test.dart' show pumpApp;
import 'support/real_http.dart';

void main() {
  testWidgets('export the open output as Word to a chosen path', (tester) async {
    final state = await pumpApp(tester);
    await tester.enterText(find.byKey(const Key('prompt')), 'Assess PR 482');
    await tester.tap(find.byKey(const Key('run')));
    await tester.pumpAndSettle();
    expect(state.thread, isNotNull);
    final dir = await tester.runAsync(() => Directory.systemTemp.createTemp('tessivo-export'));
    addTearDown(() => dir!.deleteSync(recursive: true));
    final path = '${dir!.path}${Platform.pathSeparator}risk.docx';

    await tester.tap(find.byKey(const Key('export-output')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('export-docx')));
    await tester.pumpAndSettle();
    final field = find.byType(TextField).last;
    expect(tester.widget<TextField>(field).controller!.text, endsWith('.docx'));
    await tester.enterText(field, path);
    await tester.tap(find.text('OK'));
    await tester.pumpAndSettle();
    expect(File(path).readAsStringSync(), startsWith('PK'));
    expect(find.textContaining('Saved'), findsOneWidget);
  });

  test('http client downloads an export', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    final seen = <String>[];
    server.listen((req) async {
      seen.add('${req.uri.path}?${req.uri.query}');
      req.response.headers.contentType = ContentType('application', 'octet-stream');
      req.response.add(utf8.encode('PK-bytes'));
      await req.response.close();
    });
    final client = HttpCoreClient(
      Endpoint(network: 'tcp', address: '127.0.0.1:${server.port}', token: 't'),
      client: realHttpClient(),
    );
    final bytes = await client.exportOutput('ops', 'o1', 'docx');
    expect(utf8.decode(bytes), 'PK-bytes');
    expect(seen.single, '/v1/notebooks/ops/outputs/o1/export?format=docx');
    client.close();
    await server.close(force: true);
  });
}
