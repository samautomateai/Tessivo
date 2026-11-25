import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:tessivo_desktop/src/app_state.dart';
import 'package:tessivo_desktop/src/core/http_core_client.dart';
import 'package:tessivo_desktop/src/core/models.dart';

import 'notebook_page_test.dart' show pumpPage;
import 'support/fake_core/fake_core.dart';
import 'support/notebook_tabs.dart';
import 'support/real_http.dart';

FakeCoreClient _core(AppState s) => s.core as FakeCoreClient;

Future<void> _clearToasts(WidgetTester tester) async {
  tester.state<ScaffoldMessengerState>(find.byType(ScaffoldMessenger).first).clearSnackBars();
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('save, switch and delete profiles; the status bar shows the active one', (tester) async {
    final state = await pumpPage(tester);
    state.openSettings(SettingsSection.profiles);
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('no-profiles')), findsOneWidget);
    for (final name in ['work', 'home']) {
      await tester.tap(find.byKey(const Key('profile-save')));
      await tester.pumpAndSettle();
      await tester.enterText(find.byType(TextField).last, name);
      await tester.tap(find.text('OK'));
      await tester.pumpAndSettle();
      await _clearToasts(tester);
    }
    expect(state.activeProfile, 'home');
    expect(find.text('Profile: home'), findsOneWidget);
    expect(find.byKey(const Key('profile-use-home')), findsNothing); // the active one has no Use

    await tester.tap(find.byKey(const Key('profile-use-work')));
    await tester.pumpAndSettle();
    expect(state.activeProfile, 'work');
    expect(find.text('Using work'), findsOneWidget);
    await _clearToasts(tester);

    await tester.tap(find.byKey(const Key('profile-delete-work')));
    await tester.pumpAndSettle();
    expect(state.activeProfile, '');
    expect(find.byKey(const Key('status-profile')), findsNothing);
  });

  testWidgets('a bad profile name is refused with a message', (tester) async {
    final state = await pumpPage(tester);
    state.openSettings(SettingsSection.profiles);
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('profile-save')));
    await tester.pumpAndSettle();
    await tester.enterText(find.byType(TextField).last, 'Not OK');
    await tester.tap(find.text('OK'));
    await tester.pumpAndSettle();
    expect(find.textContaining('profile name'), findsOneWidget);
  });

  testWidgets('the Sources tab suggests a work type and switches to it', (tester) async {
    final state = await pumpPage(tester);
    final core = _core(state);
    final nb = state.selected!;
    core.demoOutputs.removeWhere((o) => o.notebookId == nb.id); // no outputs yet
    core.demoSuggestions[nb.id] = const WorkTypeSuggestion(workType: 'report', reason: 'most sources are spreadsheets');
    await openTab(tester, 'sources');
    expect(find.text('These sources look like Report work'), findsOneWidget);
    await tester.tap(find.byKey(const Key('worktype-switch')));
    await tester.pumpAndSettle();
    expect(state.selected!.workType, WorkTypeId.report);
  });

  testWidgets('Dismiss hides the suggestion; a refused switch is a notice', (tester) async {
    final state = await pumpPage(tester);
    final core = _core(state);
    final nb = state.selected!;
    core.demoSuggestions[nb.id] = const WorkTypeSuggestion(workType: 'summarize', reason: 'meeting notes');
    await openTab(tester, 'sources');
    await tester.tap(find.byKey(const Key('worktype-switch')));
    await tester.pumpAndSettle();
    expect(state.notices.last, contains('no outputs')); // this notebook has an output
    await tester.tap(find.byKey(const Key('worktype-dismiss')));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('worktype-hint')), findsNothing);
  });

  test('profile and suggestion JSON', () {
    final p = ProfilesInfo.fromJson({
      'active': 'work',
      'profiles': [
        {'name': 'work', 'providers': 2, 'defaultModel': 'smart'},
      ],
    });
    expect((p.active, p.profiles.single.providers, p.profiles.single.defaultModel), ('work', 2, 'smart'));
    expect(WorkTypeSuggestion.fromJson(const {}).workType, '');
  });

  test('http client: profiles, suggestion and work type', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    final calls = <String>[];
    server.listen((req) async {
      calls.add('${req.method} ${req.uri.path} ${await utf8.decoder.bind(req).join()}');
      final p = req.uri.path;
      req.response.write(
        jsonEncode(switch (p) {
          _ when p.endsWith('/worktype-suggestion') => {'workType': 'analyze', 'reason': 'git changes'},
          _ when p.startsWith('/v1/profiles') => {'active': 'work', 'profiles': <Object>[]},
          _ => {'id': 'nb', 'name': 'N', 'workType': 'report', 'context': <Object>[]},
        }),
      );
      await req.response.close();
    });
    final c = HttpCoreClient(
      Endpoint(network: 'tcp', address: '127.0.0.1:${server.port}', token: 't'),
      client: realHttpClient(),
    );
    expect((await c.profiles()).active, 'work');
    await c.saveProfile('work');
    expect(calls.last, 'POST /v1/profiles {"name":"work"}');
    await c.useProfile('work');
    expect(calls.last, startsWith('POST /v1/profiles/work/use'));
    await c.deleteProfile('work');
    expect(calls.last, startsWith('DELETE /v1/profiles/work'));
    expect((await c.workTypeSuggestion('nb')).reason, 'git changes');
    final nb = await c.updateNotebook('nb', workType: WorkTypeId.report);
    expect((nb.workType, calls.last), (WorkTypeId.report, 'PATCH /v1/notebooks/nb {"workType":"report"}'));
    c.close();
    await server.close(force: true);
  });
}
