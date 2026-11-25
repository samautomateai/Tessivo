import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:tessivo_desktop/src/app_state.dart';
import 'package:tessivo_desktop/src/design/components/modal.dart';
import 'package:tessivo_desktop/src/design/components/table.dart';
import 'package:tessivo_desktop/src/design/components/tags.dart';

import 'app_test.dart' show pumpApp;
import 'support/fake_core/fake_core.dart';

Future<AppState> pumpWithTeamPack(WidgetTester tester) async {
  final state = await pumpApp(tester);
  await state.core.addPack('/packs/team');
  await state.refreshMcpServers();
  await tester.pumpAndSettle();
  return state;
}

void main() {
  testWidgets('Settings → Packs lists packs, adds one, refreshes and removes', (tester) async {
    final state = await pumpApp(tester);
    state.openSettings(SettingsSection.packs);
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('no-packs')), findsOneWidget);

    await tester.tap(find.byKey(const Key('pack-add')));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const Key('pack-source')), 'https://git.example.com/team/pack.git');
    await tester.tap(find.byKey(const Key('pack-add-save')));
    await tester.pumpAndSettle();
    expect(find.byType(TvDataTable), findsOneWidget);
    expect(find.text('Team pack'), findsOneWidget);

    await tester.tap(find.text('Team pack'));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('pack-readme')), findsOneWidget);
    expect(find.textContaining('templates/broken.json'), findsOneWidget, reason: 'problems are shown');

    await tester.tap(find.byKey(const Key('packs-refresh')));
    await tester.pumpAndSettle();
    expect((state.core as FakeCoreClient).packRefreshes, 1);

    await tester.tap(find.byKey(const Key('pack-remove-team')));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('no-packs')), findsOneWidget);
  });

  testWidgets('a required pack server shows a notice until it is trusted', (tester) async {
    final state = await pumpWithTeamPack(tester);
    expect(find.byKey(const Key('required-untrusted')), findsOneWidget);
    await tester.tap(find.byKey(const Key('required-untrusted-open')));
    await tester.pumpAndSettle();
    expect(state.settingsSection, SettingsSection.mcp);

    await tester.tap(find.byKey(const Key('mcp-trust-team-github')));
    await tester.pumpAndSettle();
    final modal = find.byType(TvModal<bool>);
    expect(
      find.descendant(of: modal, matching: find.text('npx -y @modelcontextprotocol/server-github')),
      findsOneWidget,
      reason: 'shows what runs',
    );
    await tester.tap(find.byKey(const Key('mcp-trust-confirm')));
    await tester.pumpAndSettle();
    expect(state.mcpServers.firstWhere((s) => s.name == 'team/github').trusted, isTrue);
    expect(find.byKey(const Key('required-untrusted')), findsNothing);
  });

  testWidgets('pack templates show their pack and mark in the gallery', (tester) async {
    final state = await pumpWithTeamPack(tester);
    state.go(Section.templates);
    await tester.pumpAndSettle();
    final card = find.byKey(const Key('template-card-team/risk-review'));
    expect(card, findsOneWidget);
    expect(find.descendant(of: card, matching: find.widgetWithText(TvTag, 'Required')), findsOneWidget);
    expect(find.descendant(of: card, matching: find.textContaining('Team pack')), findsOneWidget);
  });

  testWidgets('at 200 % zoom the packs section does not overflow', (tester) async {
    final state = await pumpWithTeamPack(tester);
    state.openSettings(SettingsSection.packs);
    tester.view.physicalSize = const Size(1366, 768);
    await state.setZoom(2.0);
    await tester.pumpAndSettle();
    expect(tester.takeException(), isNull);
    state.openSettings(SettingsSection.mcp);
    await tester.pumpAndSettle();
    expect(tester.takeException(), isNull);
  });
}
