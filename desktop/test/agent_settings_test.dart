import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:tessivo_desktop/main.dart';
import 'package:tessivo_desktop/src/app_state.dart';
import 'package:tessivo_desktop/src/core/core_client.dart';
import 'package:tessivo_desktop/src/core/settings_draft.dart';

Future<DemoCoreClient> _open(WidgetTester tester) async {
  tester.view.physicalSize = const Size(1600, 1200);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  final core = DemoCoreClient(tick: Duration.zero);
  final state = AppState(core);
  await tester.pumpWidget(TessivoApp(state: state));
  await state.load();
  await tester.pumpAndSettle();
  await tester.tap(find.text('Settings'));
  await tester.pumpAndSettle();
  await tester.scrollUntilVisible(
    find.byKey(const Key('agent-save')),
    200,
    scrollable: find.descendant(of: find.byKey(const Key('settings-list')), matching: find.byType(Scrollable)).first,
  );
  return core;
}

void main() {
  testWidgets('agent limits and tool rules are saved to the config', (tester) async {
    final core = await _open(tester);
    await tester.enterText(find.byKey(const Key('agent-max_steps')), '12');
    await tester.enterText(find.byKey(const Key('agent-max_tool_calls')), '30');
    await tester.tap(find.byKey(const Key('tool-add')));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const Key('tool-name-0')), 'read_file');
    await tester.tap(find.byKey(const Key('tool-rule-0')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('allow').last);
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('tool-add')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('agent-save')));
    await tester.pumpAndSettle();
    final d = SettingsDraft.fromJson((await core.getSettings()).raw);
    expect((d.agentLimit('max_steps'), d.agentLimit('max_tool_calls'), d.agentLimit('max_minutes')), (12, 30, null));
    expect(d.toolRules, {'read_file': 'allow'});
    expect(find.text('Saved'), findsOneWidget);
  });

  testWidgets('bad limits and duplicate rules are refused before saving', (tester) async {
    final core = await _open(tester);
    await tester.enterText(find.byKey(const Key('agent-max_steps')), 'many');
    await tester.tap(find.byKey(const Key('agent-save')));
    await tester.pumpAndSettle();
    expect(find.text('Limits must be whole numbers'), findsOneWidget);
    await tester.enterText(find.byKey(const Key('agent-max_steps')), '');
    for (var i = 0; i < 2; i++) {
      await tester.tap(find.byKey(const Key('tool-add')));
      await tester.pumpAndSettle();
      await tester.enterText(find.byKey(Key('tool-name-$i')), 'grep');
    }
    await tester.tap(find.byKey(const Key('agent-save')));
    await tester.pumpAndSettle();
    expect(find.text('Tool "grep" has two rules'), findsOneWidget);
    await tester.tap(find.byKey(const Key('tool-remove-1')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('agent-save')));
    await tester.pumpAndSettle();
    expect(SettingsDraft.fromJson((await core.getSettings()).raw).toolRules, {'grep': 'ask'});
  });
}
