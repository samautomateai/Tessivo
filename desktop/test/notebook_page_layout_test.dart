import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:tessivo_desktop/main.dart';
import 'package:tessivo_desktop/src/app_state.dart';
import 'package:tessivo_desktop/src/design/components/buttons.dart';
import 'package:tessivo_desktop/src/design/components/headers.dart';
import 'package:tessivo_desktop/src/design/components/tags.dart';

import 'support/fake_core/fake_core.dart';
import 'support/notebook_tabs.dart';

Future<AppState> pumpNotebook(WidgetTester tester, {Size size = const Size(1600, 1000), FakeCoreClient? core}) async {
  tester.view.physicalSize = size;
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  final state = AppState(core ?? FakeCoreClient(tick: Duration.zero));
  await tester.pumpWidget(TessivoApp(state: state));
  await state.load();
  await tester.pumpAndSettle();
  return state;
}

Finder inHeader(Finder f) => find.descendant(of: find.byType(TvPageHeader), matching: f);

void main() {
  testWidgets('the header shows title, work type, size and model, and one primary Run', (tester) async {
    final state = await pumpNotebook(tester);
    final nb = state.selected!;
    expect(inHeader(find.text(nb.name)), findsOneWidget);
    expect(inHeader(find.byType(WorkTypeTag)), findsOneWidget);
    expect(inHeader(find.text('≈ ${nb.enabledTokens ~/ 1000}k tokens · ${state.defaultModel}')), findsOneWidget);
    final primary = tester
        .widgetList<TvButton>(inHeader(find.byType(TvButton)))
        .where((b) => b.kind == TvButtonKind.primary)
        .toList();
    expect([for (final b in primary) b.label], ['Run']);
  });

  testWidgets('tabs show their counts', (tester) async {
    final state = await pumpNotebook(tester);
    expect(find.text('Sources · ${state.selected!.context.length}'), findsOneWidget);
    expect(find.text('Memory'), findsOneWidget);
    await state.setMemory(['Audience is the CFO', 'Use British spelling']);
    await tester.pumpAndSettle();
    expect(find.text('Memory · 2'), findsOneWidget);
    for (final t in ['Output', 'Prompt', 'Agent']) {
      expect(find.text(t), findsOneWidget);
    }
  });

  testWidgets('the output toolbar labels its actions; Compare only for revisions', (tester) async {
    await pumpNotebook(tester);
    for (final label in ['Run again', 'Edit', 'Export']) {
      expect(find.text(label), findsOneWidget, reason: label);
    }
    expect(find.text('Compare'), findsNothing);
    await tester.tap(find.byKey(const Key('new-version')));
    await tester.pumpAndSettle();
    expect(find.text('Compare'), findsOneWidget);
  });

  testWidgets('the thread toggle hides and shows the panel and says so', (tester) async {
    final state = await pumpNotebook(tester);
    final toggle = find.byKey(const Key('toggle-thread'));
    expect(tester.widget<TvIconButton>(toggle).tooltip, 'Hide thread (Ctrl J)');
    await tester.tap(toggle);
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('secondary-panel')), findsNothing);
    expect(tester.widget<TvIconButton>(toggle).tooltip, 'Show thread (Ctrl J)');
    expect(state.layout.panelOpen, isFalse);
    await tester.sendKeyDownEvent(LogicalKeyboardKey.controlLeft);
    await tester.sendKeyEvent(LogicalKeyboardKey.keyJ);
    await tester.sendKeyUpEvent(LogicalKeyboardKey.controlLeft);
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('output-thread')), findsOneWidget);
  });

  testWidgets('empty tabs say what to do', (tester) async {
    final state = await pumpNotebook(tester);
    state.select('nb-3');
    await tester.pumpAndSettle();
    expect(find.text('Run the prompt to get the first output.'), findsOneWidget);
    await openTab(tester, 'sources');
    expect(find.textContaining('Add files, folders, web pages or git changes'), findsOneWidget);
    await openTab(tester, 'memory');
    expect(find.textContaining('Facts the model should always know'), findsOneWidget);
    await openTab(tester, 'agent');
    expect(find.textContaining('None yet.'), findsOneWidget);
  });

  testWidgets('with no model set up the page offers Connect a model, which opens Settings', (tester) async {
    final core = FakeCoreClient(tick: Duration.zero);
    await core.updateSettings({'providers': const <Object?>[], 'aliases': const <Object?>[]});
    final state = await pumpNotebook(tester, core: core);
    expect(find.byKey(const Key('setup-banner')), findsOneWidget);
    expect(find.text('Connect a model'), findsWidgets);
    await tester.tap(find.byKey(const Key('setup-open')));
    await tester.pumpAndSettle();
    expect(state.section, Section.settings);
  });

  testWidgets('with no notebooks the page offers New notebook and Start from a template', (tester) async {
    final state = await pumpNotebook(tester);
    for (final nb in [...state.notebooks]) {
      await state.deleteNotebook(nb.id);
    }
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('no-notebooks')), findsOneWidget);
    expect(find.text('New notebook'), findsOneWidget);
    await tester.tap(find.text('Start from a template'));
    await tester.pumpAndSettle();
    expect(state.section, Section.templates);
  });

  testWidgets('a 200-character name truncates in the header, side bar and breadcrumbs', (tester) async {
    final state = await pumpNotebook(tester);
    final name = ('Quarterly vendor risk ' * 10).substring(0, 200);
    await state.renameNotebook(state.selectedId!, name);
    await tester.pumpAndSettle();
    expect(tester.takeException(), isNull);
    final header = tester.widget<Text>(inHeader(find.text(name)));
    expect((header.maxLines, header.overflow), (1, TextOverflow.ellipsis));
    expect(
      find.descendant(of: find.byKey(const Key('side-bar')), matching: find.textContaining('Quarterly')),
      findsOneWidget,
    );
    final crumbs = tester.widget<Text>(find.byKey(const Key('breadcrumbs')));
    expect((crumbs.data, crumbs.maxLines, crumbs.overflow), ('Notebooks / $name', 1, TextOverflow.ellipsis));
  });

  testWidgets('at 200 % zoom on a 1366×768 window no tab overflows', (tester) async {
    final state = await pumpNotebook(tester, size: const Size(1366, 768));
    await state.setZoom(2.0);
    await tester.pumpAndSettle();
    expect(tester.takeException(), isNull);
    for (final tab in ['sources', 'prompt', 'memory', 'agent', 'output']) {
      await openTab(tester, tab);
      expect(tester.takeException(), isNull, reason: tab);
    }
    await state.togglePanel(width: 683);
    await tester.pumpAndSettle();
    expect(tester.takeException(), isNull, reason: 'thread open at 200 %');
  });
}
