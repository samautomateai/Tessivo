import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:tessivo_desktop/main.dart';
import 'package:tessivo_desktop/src/app_state.dart';
import 'package:tessivo_desktop/src/core/core_client.dart';
import 'package:tessivo_desktop/src/core/models.dart';
import 'package:tessivo_desktop/src/theme.dart';

Future<AppState> pumpApp(WidgetTester tester) async {
  tester.view.physicalSize = const Size(1600, 1000);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  final state = AppState(DemoCoreClient(tick: Duration.zero));
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
    expect(find.text('payments-service'), findsOneWidget);
    expect(find.text('≈ 51k tokens'), findsOneWidget);
  });

  testWidgets('toggling context updates the token estimate', (tester) async {
    await pumpApp(tester);
    await tester.tap(find.byKey(const Key('ctx-c1')));
    await tester.pumpAndSettle();
    expect(find.text('≈ 13k tokens'), findsOneWidget);
  });

  testWidgets('running streams output and shows the outcome', (tester) async {
    final state = await pumpApp(tester);
    await tester.enterText(find.byKey(const Key('prompt')), 'Assess PR 482');
    await tester.tap(find.byKey(const Key('run')));
    await tester.pumpAndSettle();
    expect(state.running, isFalse);
    expect(state.outcome?.valid, isTrue);
    expect(find.textContaining('Risk score'), findsOneWidget);
    expect(find.text('Validated ✓'), findsOneWidget);
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
    expect(find.textContaining('No context yet'), findsOneWidget);
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
    await tester.tap(find.text('Outputs library'));
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

  testWidgets('settings lets you pick thinking effort', (tester) async {
    await pumpApp(tester);
    await tester.tap(find.text('Settings'));
    await tester.pumpAndSettle();
    expect(find.text('Fallback chain for "smart"'), findsOneWidget);
    await tester.tap(find.text('high'));
    await tester.pumpAndSettle();
    final button = tester.widget<SegmentedButton<String>>(find.byKey(const Key('thinking')));
    expect(button.selected, {'high'});
    await tester.tap(find.text('Weekly ops report'));
    await tester.pumpAndSettle();
    expect(find.textContaining('No context yet'), findsOneWidget);
  });

  test('theme and labels cover every work type', () {
    for (final t in WorkTypeId.values) {
      expect(workTypeLabel(t), isNotEmpty);
      expect(colorsFor(t).dot, isNot(colorsFor(t).background));
    }
    expect(buildTheme(Brightness.dark).brightness, Brightness.dark);
  });
}
