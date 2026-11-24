import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:tessivo_desktop/main.dart';
import 'package:tessivo_desktop/src/app_state.dart';
import 'package:tessivo_desktop/src/core/http_core_client.dart';

import 'support/fake_core/fake_core.dart';
import 'support/real_http.dart';

void main() {
  testWidgets('Test on an alias shows which models answer', (tester) async {
    tester.view.physicalSize = const Size(1600, 1200);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    final core = FakeCoreClient(tick: Duration.zero);
    final state = AppState(core);
    await tester.pumpWidget(TessivoApp(state: state));
    await state.load();
    await tester.pumpAndSettle();
    await tester.tap(find.text('Settings'));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('test-alias-smart')));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('model-check-litellm/primary-model')), findsOneWidget);
    expect(find.textContaining('Answers in 120 ms'), findsNWidgets(2));
    await tester.tap(find.text('Close'));
    await tester.pumpAndSettle();
    expect(find.byType(AlertDialog), findsNothing);
  });

  test('demo checks: offline models fail, unknown names are refused', () async {
    final core = FakeCoreClient(tick: Duration.zero);
    final got = await core.testModel('lab/offline-model');
    expect((got.single.ok, got.single.error), (false, 'connection refused'));
    expect(() => core.testModel('nope'), throwsA(isA<CoreException>()));
  });

  test('http client posts the model and reads the checks', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    String? body;
    server.listen((req) async {
      body = await utf8.decoder.bind(req).join();
      req.response.write(
        '{"checks":[{"model":"a/x","ok":true,"millis":7,"reply":"OK"},{"model":"a/y","ok":false,"error":"401"}]}',
      );
      await req.response.close();
    });
    final client = HttpCoreClient(
      Endpoint(network: 'tcp', address: '127.0.0.1:${server.port}', token: 't'),
      client: realHttpClient(),
    );
    final got = await client.testModel('smart');
    client.close();
    await server.close(force: true);
    expect(body, '{"model":"smart"}');
    expect([for (final c in got) (c.model, c.ok, c.millis, c.error)], [('a/x', true, 7, ''), ('a/y', false, 0, '401')]);
  });
}
