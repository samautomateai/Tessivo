import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:tessivo_desktop/src/core/http_core_client.dart';

import 'app_test.dart' show pumpApp;
import 'support/notebook_tabs.dart';
import 'support/real_http.dart';

void main() {
  testWidgets('a library prompt fills the prompt box, own work type first', (tester) async {
    final state = await pumpApp(tester);
    await openTab(tester, 'prompt');
    await openTab(tester, 'prompt');
    await tester.tap(find.byKey(const Key('prompts-menu')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('prompt-library')));
    await tester.pumpAndSettle();
    final own = state.selected!.workType.name;
    final tiles = tester.widgetList<ListTile>(
      find.descendant(of: find.byType(AlertDialog), matching: find.byType(ListTile)),
    );
    expect((tiles.first.trailing! as Text).data, own, reason: 'the notebook work type comes first');
    await tester.tap(find.byKey(const Key('library-action-items')));
    await tester.pumpAndSettle();
    final prompt = tester.widget<TextField>(find.byKey(const Key('prompt')));
    expect(prompt.controller!.text, startsWith('List every action item'));
  });

  test('http client asks for one work type', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    String? path;
    server.listen((req) async {
      path = req.uri.toString();
      req.response.write('{"prompts":[{"id":"a","name":"A","workType":"report","text":"Write"}]}');
      await req.response.close();
    });
    final client = HttpCoreClient(
      Endpoint(network: 'tcp', address: '127.0.0.1:${server.port}', token: 't'),
      client: realHttpClient(),
    );
    final got = await client.promptLibrary(workType: 'report');
    client.close();
    await server.close(force: true);
    expect(path, '/v1/prompt-library?workType=report');
    expect([for (final p in got) (p.id, p.workType, p.text)], [('a', 'report', 'Write')]);
  });
}
