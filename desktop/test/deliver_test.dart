import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:tessivo_desktop/src/app_state.dart';
import 'package:tessivo_desktop/src/core/http_core_client.dart';
import 'package:tessivo_desktop/src/core/models.dart';
import 'package:tessivo_desktop/src/design/components/buttons.dart';

import 'notebook_page_test.dart' show pumpPage;
import 'support/fake_core/fake_core.dart';
import 'support/real_http.dart';

FakeCoreClient _core(AppState s) => s.core as FakeCoreClient;

const _gh = IntegrationInfo(name: 'gh', kind: 'github', project: 'acme/shop', tokenRef: 'GH');
const _share = IntegrationInfo(name: 'share', kind: 'folder', path: '/srv/share');

Future<void> _send(WidgetTester tester, String item) async {
  await tester.tap(find.byKey(const Key('send')));
  await tester.pumpAndSettle();
  await tester.tap(find.byKey(Key(item)).last);
  await tester.pumpAndSettle();
}

/// Drops toasts already shown, so the next one appears at once.
Future<void> _clearToasts(WidgetTester tester) async {
  tester.state<ScaffoldMessengerState>(find.byType(ScaffoldMessenger).first).clearSnackBars();
  await tester.pumpAndSettle();
}

bool _ticked(WidgetTester tester, int i) =>
    tester.widget<Checkbox>(find.descendant(of: find.byKey(Key('issue-$i')), matching: find.byType(Checkbox))).value!;

void main() {
  testWidgets('findings become issues; sent ones are marked and start unticked', (tester) async {
    final state = await pumpPage(tester);
    _core(state).demoIntegrations.add(_gh);
    await _send(tester, 'send-issues');
    expect(find.text('Retries are not idempotent'), findsOneWidget);
    expect(_ticked(tester, 0) && _ticked(tester, 1), isTrue);
    await tester.tap(find.byKey(const Key('issues-push')));
    await tester.pumpAndSettle();
    expect(find.text('Created 2 issues in gh'), findsOneWidget);
    await _clearToasts(tester);

    await _send(tester, 'send-issues');
    expect(find.text('Sent'), findsNWidgets(2));
    expect(_ticked(tester, 0) || _ticked(tester, 1), isFalse);
    expect(tester.widget<TvButton>(find.byKey(const Key('issues-push'))).onPressed, isNull);
    await tester.tap(find.byKey(const Key('issue-0')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('issues-push')));
    await tester.pumpAndSettle();
    expect(find.text('Created 1 issue in gh'), findsOneWidget); // a ticked, sent finding goes again
  });

  testWidgets('a tracker that refuses shows why and keeps the dialog open', (tester) async {
    final state = await pumpPage(tester);
    _core(state)
      ..demoIntegrations.add(_gh)
      ..failingIntegration = 'gh';
    await _send(tester, 'send-issues');
    await tester.tap(find.byKey(const Key('issues-push')));
    await tester.pumpAndSettle();
    expect(find.text('Not sent'), findsOneWidget);
    expect(find.textContaining('401 Unauthorized'), findsOneWidget);
    expect(find.byKey(const Key('issues-dialog')), findsOneWidget);
  });

  testWidgets('with no integration the dialogs point to Settings', (tester) async {
    final state = await pumpPage(tester);
    await _send(tester, 'send-publish');
    expect(find.text('Nothing to send to yet'), findsOneWidget);
    await tester.tap(find.byKey(const Key('open-integrations')));
    await tester.pumpAndSettle();
    expect((state.section, state.settingsSection), (Section.settings, SettingsSection.integrations));
  });

  testWidgets('publish sends the output to a folder, Confluence or Notion', (tester) async {
    final state = await pumpPage(tester);
    _core(state).demoIntegrations.addAll([_gh, _share]);
    await _send(tester, 'send-publish');
    expect(find.textContaining('gh ·'), findsNothing); // trackers take no pages
    await tester.tap(find.byKey(const Key('publish-go')));
    await tester.pumpAndSettle();
    expect(find.text('Published to share: https://wiki.test/o1'), findsOneWidget);
  });

  testWidgets('Create issues is only for Analyze outputs', (tester) async {
    final state = await pumpPage(tester);
    state.select('nb-2'); // a Summarize notebook
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('send')));
    await tester.pumpAndSettle();
    final item = tester.widget<MenuItemButton>(find.byKey(const Key('send-issues')));
    expect(item.onPressed, isNull);
  });

  testWidgets('Settings → Integrations adds, sets the token, tests and removes', (tester) async {
    final state = await pumpPage(tester);
    state.openSettings(SettingsSection.integrations);
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('no-integrations')), findsOneWidget);
    await tester.tap(find.byKey(const Key('integration-add')));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const Key('integration-name')), 'gh');
    await tester.enterText(find.byKey(const Key('integration-project')), 'acme/shop');
    await tester.tap(find.byKey(const Key('integration-save')));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const Key('integration-token-value')), 't0ken');
    await tester.tap(find.byKey(const Key('integration-token-save')));
    await tester.pumpAndSettle();
    expect(await _core(state).secretSet('TESSIVO_GH_TOKEN'), isTrue);
    expect(find.text('Set'), findsOneWidget);
    expect(_core(state).demoIntegrations.single.project, 'acme/shop');
    await _clearToasts(tester);

    await tester.tap(find.byKey(const Key('integration-menu-gh')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('integration-test-gh')).last);
    await tester.pumpAndSettle();
    expect(find.text('gh: connected'), findsOneWidget);
    await tester.tap(find.byKey(const Key('integration-menu-gh')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('integration-remove-gh')).last);
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('no-integrations')), findsOneWidget);
  });

  testWidgets('adding a folder asks for no token; a bad name is refused', (tester) async {
    final state = await pumpPage(tester);
    state.openSettings(SettingsSection.integrations);
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('integration-add')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('integration-kind')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Folder (Markdown files)').last);
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const Key('integration-name')), 'Bad Name');
    await tester.enterText(find.byKey(const Key('integration-path')), '/srv/share');
    await tester.tap(find.byKey(const Key('integration-save')));
    await tester.pumpAndSettle();
    expect(find.textContaining('integration name'), findsOneWidget);
    expect(find.byKey(const Key('integration-token-value')), findsNothing);
  });

  testWidgets('at 200 % zoom the send dialogs do not overflow', (tester) async {
    final state = await pumpPage(tester, width: 1366);
    _core(state).demoIntegrations.addAll([_gh, _share]);
    await state.setZoom(2.0);
    await tester.pumpAndSettle();
    await _send(tester, 'send-issues');
    expect(tester.takeException(), isNull);
    await tester.tap(find.text('Cancel'));
    await tester.pumpAndSettle();
    await _send(tester, 'send-publish');
    expect(tester.takeException(), isNull);
  });

  test('integration JSON and bodies', () {
    final i = IntegrationInfo.fromJson({
      'name': 'jira',
      'kind': 'jira',
      'base_url': 'https://j.test',
      'project': 'OPS',
      'token_ref': 'T',
      'tokenSet': true,
    });
    expect((i.forIssues, i.tokenSet, i.target), (true, true, 'OPS'));
    expect(i.toBody(), {
      'name': 'jira',
      'kind': 'jira',
      'base_url': 'https://j.test',
      'project': 'OPS',
      'token_ref': 'T',
    });
    final r = PushResult.fromJson({
      'sent': [
        {'integration': 'gh', 'finding': 1, 'url': 'u'},
      ],
      'skipped': [0],
    });
    expect(r.sent.single.finding, 1);
    expect(r.skipped, [0]);
    expect(takesIssues('notion'), isFalse);
  });

  test('http client: issues, push, publish, deliveries and integrations', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    final calls = <String>[];
    server.listen((req) async {
      final body = await utf8.decoder.bind(req).join();
      calls.add('${req.method} ${req.uri.path} $body');
      final p = req.uri.path;
      final reply = switch (p) {
        _ when p.endsWith('/issues') => {
          'issues': [
            {
              'index': 0,
              'title': 'A',
              'labels': ['x'],
            },
          ],
        },
        _ when p.endsWith('/push') => {
          'sent': <Object>[],
          'skipped': [0],
        },
        _ when p.endsWith('/publish') => {'integration': 'share', 'finding': -1, 'url': 'file:///x.md'},
        _ when p.endsWith('/deliveries') => {'deliveries': <Object>[]},
        '/v1/integrations' when req.method == 'GET' => {
          'integrations': [
            {'name': 'gh', 'kind': 'github'},
          ],
        },
        '/v1/integrations' => {'name': 'gh', 'kind': 'github'},
        _ => <String, Object>{},
      };
      if (req.method == 'DELETE') {
        req.response.statusCode = 204;
      } else {
        req.response.write(jsonEncode(reply));
      }
      await req.response.close();
    });
    final c = HttpCoreClient(
      Endpoint(network: 'tcp', address: '127.0.0.1:${server.port}', token: 't'),
      client: realHttpClient(),
    );
    expect((await c.outputIssues('nb', 'o1')).single.labels, ['x']);
    expect((await c.pushIssues('nb', 'o1', 'gh', findings: [0], again: true)).skipped, [0]);
    expect(calls.last, 'POST /v1/notebooks/nb/outputs/o1/issues/push {"integration":"gh","findings":[0],"again":true}');
    expect((await c.publishOutput('nb', 'o1', 'share')).url, 'file:///x.md');
    expect(await c.deliveries('nb', 'o1'), isEmpty);
    expect((await c.listIntegrations()).single.name, 'gh');
    await c.addIntegration(_gh);
    expect(calls.last, contains('"project":"acme/shop"'));
    await c.testIntegration('gh');
    expect(calls.last, startsWith('POST /v1/integrations/gh/test'));
    await c.removeIntegration('gh');
    expect(calls.last, startsWith('DELETE /v1/integrations/gh'));
    c.close();
    await server.close(force: true);
  });
}
