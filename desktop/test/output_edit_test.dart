import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:tessivo_desktop/src/core/http_core_client.dart';
import 'package:tessivo_desktop/src/widgets/output_edit_form.dart';

import 'app_test.dart' show pumpApp;
import 'support/real_http.dart';

void main() {
  testWidgets('edit an output in a form and save it as a new version', (tester) async {
    final state = await pumpApp(tester);
    await tester.enterText(find.byKey(const Key('prompt')), 'Assess PR 482');
    await tester.tap(find.byKey(const Key('run')));
    await tester.pumpAndSettle();
    final parent = state.thread!.outputId;
    await tester.tap(find.byKey(const Key('edit-output')));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const Key('edit-field-title')), 'Fixed title');
    await tester.enterText(find.byKey(const Key('edit-field-key_points')), 'One\n\n  Two  \n');
    await tester.tap(find.byKey(const Key('edit-save')));
    await tester.pumpAndSettle();
    final v = state.outputs.firstWhere((o) => o.id == state.thread!.outputId);
    expect((v.title, v.parentId, v.trigger), ('Fixed title', parent, 'Edited'));
    expect(find.text('Saved as a new version'), findsOneWidget);
  });

  testWidgets('an empty title is refused and reported', (tester) async {
    final state = await pumpApp(tester);
    await tester.enterText(find.byKey(const Key('prompt')), 'Assess PR 482');
    await tester.tap(find.byKey(const Key('run')));
    await tester.pumpAndSettle();
    final before = state.outputs.length;
    await tester.tap(find.byKey(const Key('edit-output')));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const Key('edit-field-title')), ' ');
    await tester.tap(find.byKey(const Key('edit-save')));
    await tester.pumpAndSettle();
    expect(find.text('the edited output needs a title'), findsOneWidget);
    expect(state.outputs.length, before);
  });

  testWidgets('the form edits lists by line and nested fields as checked JSON', (tester) async {
    Map<String, Object?>? saved;
    await tester.pumpWidget(
      MaterialApp(
        home: Builder(
          builder: (context) => TextButton(
            onPressed: () async => saved = await showDialog<Map<String, Object?>>(
              context: context,
              builder: (_) => const OutputEditDialog(
                data: {
                  'title': 'T',
                  'risks': ['a'],
                  'per_source': [
                    {'source': 'x', 'summary': 's'},
                  ],
                },
              ),
            ),
            child: const Text('open'),
          ),
        ),
      ),
    );
    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();
    expect(find.text('Per source'), findsOneWidget);
    await tester.enterText(find.byKey(const Key('edit-field-per_source')), '[{"source": ');
    await tester.tap(find.byKey(const Key('edit-save')));
    await tester.pumpAndSettle();
    expect(find.text('Not valid JSON'), findsOneWidget);
    await tester.enterText(find.byKey(const Key('edit-field-per_source')), '[{"source": "y"}]');
    await tester.enterText(find.byKey(const Key('edit-field-risks')), 'b\n c ');
    await tester.tap(find.byKey(const Key('edit-save')));
    await tester.pumpAndSettle();
    expect(saved, {
      'title': 'T',
      'risks': ['b', 'c'],
      'per_source': [
        {'source': 'y'},
      ],
    });
  });

  test('http client posts the edited data', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    String? body;
    server.listen((req) async {
      body = '${req.method} ${req.uri.path} ${await utf8.decoder.bind(req).join()}';
      req.response.statusCode = 201;
      req.response.write(
        jsonEncode({'id': 'o2', 'title': 'Fixed', 'workType': 'summarize', 'parentId': 'o1', 'version': 2}),
      );
      await req.response.close();
    });
    final client = HttpCoreClient(
      Endpoint(network: 'tcp', address: '127.0.0.1:${server.port}', token: 't'),
      client: realHttpClient(),
    );
    final v = await client.editOutput('ops', 'o1', {'title': 'Fixed'});
    client.close();
    await server.close(force: true);
    expect(body, 'POST /v1/notebooks/ops/outputs/o1/edits {"data":{"title":"Fixed"}}');
    expect((v.id, v.parentId, v.version), ('o2', 'o1', 2));
  });
}
