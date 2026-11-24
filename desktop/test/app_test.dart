import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:tessivo_desktop/main.dart';
import 'package:tessivo_desktop/src/app_state.dart';
import 'package:tessivo_desktop/src/core/models.dart';
import 'package:tessivo_desktop/src/design/colors.dart';
import 'package:tessivo_desktop/src/design/theme.dart' show tvTheme;
import 'package:tessivo_desktop/src/design/work_types.dart';

import 'support/fake_core/fake_core.dart';
import 'support/notebook_tabs.dart';
import 'support/settings_sections.dart';

Future<AppState> pumpApp(WidgetTester tester) async {
  tester.view.physicalSize = const Size(1600, 1000);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  final state = AppState(FakeCoreClient(tick: Duration.zero));
  await tester.pumpWidget(TessivoApp(state: state));
  expect(find.byType(CircularProgressIndicator), findsOneWidget);
  await state.load();
  await tester.pumpAndSettle();
  return state;
}

void main() {
  testWidgets('shows notebooks and the selected workspace', (tester) async {
    await pumpApp(tester);
    expect(find.text('Tessivo'), findsOneWidget);
    expect(find.text('Change risk review'), findsWidgets);
    expect(find.text('Weekly ops report'), findsOneWidget);
    expect(find.text('Mon 08:00'), findsOneWidget);
    expect(find.text('Output'), findsOneWidget);
    await openTab(tester, 'sources');
    expect(find.text('payments-service'), findsOneWidget);
    expect(find.textContaining('≈ 51k tokens · smart'), findsOneWidget);
  });

  testWidgets('toggling context updates the token estimate', (tester) async {
    await pumpApp(tester);
    await openTab(tester, 'sources');
    await tester.tap(find.byKey(const Key('ctx-c1')));
    await tester.pumpAndSettle();
    expect(find.textContaining('≈ 13k tokens ·'), findsOneWidget);
  });

  testWidgets('running streams output and shows the outcome', (tester) async {
    final state = await pumpApp(tester);
    await openTab(tester, 'prompt');
    await tester.enterText(find.byKey(const Key('prompt')), 'Assess PR 482');
    await tester.tap(find.byKey(const Key('run')));
    await tester.pumpAndSettle();
    expect(state.running, isFalse);
    expect(state.outcome?.valid, isTrue);
    expect(find.textContaining('Risk score'), findsOneWidget);
    expect(find.text('Validated'), findsOneWidget);
    expect(find.textContaining('fell back to model B'), findsOneWidget);
  });

  testWidgets('new notebook dialog validates and creates', (tester) async {
    final state = await pumpApp(tester);
    await tester.tap(find.byKey(const Key('new-notebook')));
    await tester.pumpAndSettle();
    expect(find.text('What kind of work is this notebook for?'), findsOneWidget);
    await tester.tap(find.byKey(const Key('type-summarize')));
    await tester.pumpAndSettle();
    expect(find.textContaining('Transcripts'), findsOneWidget);
    await tester.tap(find.byKey(const Key('create-notebook')));
    await tester.pumpAndSettle();
    expect(find.text('Enter a name'), findsOneWidget);
    await tester.enterText(find.byKey(const Key('notebook-name')), 'Board minutes');
    await tester.tap(find.byKey(const Key('create-notebook')));
    await tester.pumpAndSettle();
    expect(state.selected?.name, 'Board minutes');
    expect(state.selected?.workType, WorkTypeId.summarize);
    await openTab(tester, 'sources');
    expect(find.textContaining('No sources yet'), findsOneWidget);
  });

  testWidgets('cancel closes the dialog', (tester) async {
    await pumpApp(tester);
    await tester.tap(find.byKey(const Key('new-notebook')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Cancel'));
    await tester.pumpAndSettle();
    expect(find.byType(AlertDialog), findsNothing);
  });

  testWidgets('library filters by work type', (tester) async {
    await pumpApp(tester);
    await tester.tap(find.byKey(const Key('activity-library')));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('output-o1')), findsOneWidget);
    expect(find.byKey(const Key('output-o2')), findsOneWidget);
    await tester.tap(find.byKey(const Key('filter-report')));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('output-o1')), findsNothing);
    expect(find.byKey(const Key('output-o2')), findsOneWidget);
    await tester.tap(find.byKey(const Key('filter-document')));
    await tester.pumpAndSettle();
    expect(find.text('No outputs yet.'), findsOneWidget);
    await tester.tap(find.byKey(const Key('filter-all')));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('output-o3')), findsOneWidget);
  });

  testWidgets('settings shows aliases, defaults, providers and models', (tester) async {
    await pumpApp(tester);
    await tester.tap(find.text('Settings'));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('alias-smart')), findsOneWidget);
    await openSettingsSection(tester, 'defaults');
    expect(tester.widget<TextField>(find.byKey(const Key('default-model'))).controller!.text, 'smart');
    expect(find.text('Thinking: medium'), findsOneWidget);
    await openSettingsSection(tester, 'models');
    expect(find.text('Stored in your OS keychain as TESSIVO_LITELLM_KEY'), findsOneWidget);
    expect(find.text('Key set'), findsOneWidget);
    await tester.tap(find.byKey(const Key('load-models-litellm')));
    await tester.pumpAndSettle();
    expect(find.text('primary-model'), findsOneWidget);
    expect(find.text('Refresh'), findsOneWidget);
    await tester.tap(find.byKey(const Key('activity-notebooks')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Weekly ops report'));
    await tester.pumpAndSettle();
    await openTab(tester, 'sources');
    expect(find.textContaining('No sources yet'), findsOneWidget);
  });

  test('theme and labels cover every work type', () {
    for (final t in WorkTypeId.values) {
      expect(workTypeLabel(t), isNotEmpty);
      expect(TvColors.dark.tag(t).dot, isNot(TvColors.dark.tag(t).background));
    }
    expect(tvTheme(TvColors.dark).brightness, Brightness.dark);
  });
}
