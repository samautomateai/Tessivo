import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:tessivo_desktop/src/core/http_core_client.dart';

import 'app_test.dart' show pumpApp;
import 'support/real_http.dart';

void main() {
  testWidgets('run an output again on another model', (tester) async {
    final state = await pumpApp(tester);
    await tester.enterText(find.byKey(const Key('prompt')), 'Assess PR 482');
    await tester.tap(find.byKey(const Key('run')));
    await tester.pumpAndSettle();
    final before = state.outputs.length;
    await tester.tap(find.byKey(const Key('rerun')));
    await tester.pumpAndSettle();
    final model = find.byKey(const Key('rerun-model'));
    expect(tester.widget<TextField>(model).controller!.text, isNotEmpty);
    await tester.enterText(model, 'litellm/other');
    await tester.tap(find.byKey(const Key('rerun-go')));
    await tester.pumpAndSettle();
    expect((state.lastModel, state.running, state.outcome?.valid), ('litellm/other', false, true));
    expect(before, isPositive);
  });

  test('http client reads the run behind an output and runs on a model', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    final bodies = <String>[];
    server.listen((req) async {
      bodies.add(await utf8.decoder.bind(req).join());
      if (req.method == 'GET') {
        req.response.write(
          jsonEncode({
            'runId': 'r1',
            'prompt': 'Summarize',
            'vars': {'w': '1'},
            'model': 'smart',
          }),
        );
      } else {
        req.response.headers.contentType = ContentType('text', 'event-stream');
        req.response.write('event: status\ndata: {"text":"ok"}\n\n');
      }
      await req.response.close();
    });
    final client = HttpCoreClient(
      Endpoint(network: 'tcp', address: '127.0.0.1:${server.port}', token: 't'),
      client: realHttpClient(),
    );
    final ri = await client.outputRun('ops', 'o1');
    expect((ri.prompt, ri.vars['w'], ri.model), ('Summarize', '1', 'smart'));
    await client.startRun(notebookId: 'ops', prompt: 'p', vars: const {}, model: 'litellm/x').toList();
    expect(bodies.last, contains('"overrides":{"model":"litellm/x"}'));
    client.close();
    await server.close(force: true);
  });
}
