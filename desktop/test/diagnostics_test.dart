import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:tessivo_desktop/src/core/http_core_client.dart';

import 'app_test.dart' show pumpApp;
import 'support/real_http.dart';

void main() {
  testWidgets('save diagnostics from settings', (tester) async {
    await pumpApp(tester);
    await tester.tap(find.text('Settings'));
    await tester.pumpAndSettle();
    final dir = await tester.runAsync(() => Directory.systemTemp.createTemp('tessivo-diag'));
    addTearDown(() => dir!.deleteSync(recursive: true));
    final path = '${dir!.path}${Platform.pathSeparator}diag.zip';
    await tester.ensureVisible(find.byKey(const Key('save-diagnostics')));
    await tester.tap(find.byKey(const Key('save-diagnostics')));
    await tester.pumpAndSettle();
    expect(tester.widget<TextField>(find.byType(TextField).last).controller!.text, endsWith('.zip'));
    await tester.enterText(find.byType(TextField).last, path);
    await tester.tap(find.text('OK'));
    await tester.pumpAndSettle();
    expect(File(path).lengthSync(), greaterThan(0));
    expect(find.textContaining('Saved'), findsOneWidget);
  });

  test('http client downloads diagnostics', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    server.listen((req) async {
      req.response.headers.contentType = ContentType('application', 'zip');
      req.response.add(utf8.encode('PK-zip'));
      await req.response.close();
    });
    final client = HttpCoreClient(
      Endpoint(network: 'tcp', address: '127.0.0.1:${server.port}', token: 't'),
      client: realHttpClient(),
    );
    expect(utf8.decode(await client.diagnostics()), 'PK-zip');
    client.close();
    await server.close(force: true);
  });
}
