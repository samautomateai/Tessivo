import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:tessivo_desktop/src/core/http_core_client.dart';
import 'package:tessivo_desktop/src/core/models.dart';

import 'app_test.dart' show pumpApp;
import 'support/real_http.dart';

Future<void> _menu(WidgetTester tester, String key) async {
  await tester.tap(find.byKey(const Key('prompts-menu')));
  await tester.pumpAndSettle();
  await tester.tap(find.byKey(Key(key)).last);
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('save the prompt, pick it, run with it, delete it', (tester) async {
    final state = await pumpApp(tester);
    await tester.enterText(find.byKey(const Key('prompt')), 'Assess {{change}} for risk');
    await _menu(tester, 'prompt-save');
    await tester.enterText(find.byType(TextField).last, 'risk');
    await tester.tap(find.text('OK'));
    await tester.pumpAndSettle();
    expect(state.prompts.single.name, 'risk');

    await tester.enterText(find.byKey(const Key('prompt')), 'something else');
    await _menu(tester, 'prompt-use-risk');
    expect(tester.widget<TextField>(find.byKey(const Key('prompt'))).controller!.text, 'Assess {{change}} for risk');
    expect(find.text('Prompt: risk'), findsOneWidget);
    await tester.tap(find.byKey(const Key('run')));
    await tester.pumpAndSettle();
    expect(state.lastPromptName, 'risk');

    await tester.enterText(find.byKey(const Key('prompt')), 'edited text');
    await tester.pump();
    expect(find.text('Prompt: risk'), findsNothing); // editing detaches the saved prompt

    await _menu(tester, 'prompt-delete-risk');
    expect(state.prompts, isEmpty);
  });

  test('http client lists, saves, deletes prompts and runs by name', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    final calls = <String>[];
    server.listen((req) async {
      calls.add('${req.method} ${req.uri.path} ${await utf8.decoder.bind(req).join()}');
      final res = req.response;
      switch (req.method) {
        case 'GET':
          res.write(
            jsonEncode({
              'prompts': [
                {
                  'name': 'risk',
                  'text': 'Assess',
                  'settings': {'model': 'smart'},
                },
              ],
            }),
          );
        case 'PUT':
          res.write(jsonEncode({'name': 'risk', 'text': 'Assess', 'settings': <String, Object?>{}}));
        case 'DELETE':
          res.statusCode = 204;
        default:
          res.headers.contentType = ContentType('text', 'event-stream');
          res.write('event: status\ndata: {"text":"ok"}\n\n');
      }
      await res.close();
    });
    final client = HttpCoreClient(
      Endpoint(network: 'tcp', address: '127.0.0.1:${server.port}', token: 't'),
      client: realHttpClient(),
    );
    final list = await client.listPrompts('ops');
    expect((list.single.name, list.single.model), ('risk', 'smart'));
    await client.savePrompt('ops', const SavedPrompt(name: 'risk', text: 'Assess'));
    expect(calls.last, 'PUT /v1/notebooks/ops/prompts/risk {"text":"Assess"}');
    await client.deletePrompt('ops', 'risk');
    expect(calls.last, startsWith('DELETE /v1/notebooks/ops/prompts/risk'));
    await client.startRun(notebookId: 'ops', prompt: 'Assess', vars: const {}, promptName: 'risk').toList();
    expect(calls.last, contains('"promptName":"risk"'));
    client.close();
    await server.close(force: true);
  });
}
