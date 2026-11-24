import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:tessivo_desktop/main.dart';
import 'package:tessivo_desktop/src/app_state.dart';
import 'package:tessivo_desktop/src/core/models.dart';
import 'package:tessivo_desktop/src/research_state.dart';

import 'support/fake_core/fake_core.dart';

Future<AppState> _pump(WidgetTester tester, {double width = 1600, FakeCoreClient? core}) async {
  tester.view.physicalSize = Size(width, 1000);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  final state = AppState(core ?? FakeCoreClient(tick: Duration.zero));
  await tester.pumpWidget(TessivoApp(state: state));
  await state.load();
  await tester.pumpAndSettle();
  return state;
}

Future<void> _openBoard(WidgetTester tester) async {
  await tester.tap(find.byKey(const Key('activity-research')));
  await tester.pumpAndSettle();
  await tester.tap(find.byKey(const Key('research-board-papers')));
  await tester.pumpAndSettle();
}

Future<void> _tab(WidgetTester tester, String tab) async {
  await tester.tap(find.byKey(Key('tab-$tab')));
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('with no research notebooks the page says how to start', (tester) async {
    final state = await _pump(tester);
    for (final r in [...state.research]) {
      await state.core.deleteResearch(r.id);
    }
    await state.load();
    state.go(Section.research);
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('no-research')), findsOneWidget);
    expect(find.text('No research notebooks yet'), findsOneWidget);
  });

  testWidgets('the dialog creates a research notebook listed under RESEARCH', (tester) async {
    final state = await _pump(tester);
    await tester.tap(find.byKey(const Key('activity-research')));
    await tester.pumpAndSettle();
    expect(find.text('RESEARCH'), findsOneWidget);
    expect(find.text('Board papers'), findsOneWidget);
    await tester.tap(find.byKey(const Key('new-research')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('kind-research')));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('type-summarize')), findsNothing);
    await tester.tap(find.byKey(const Key('create-notebook')));
    await tester.pumpAndSettle();
    expect(find.text('Enter a name'), findsOneWidget);
    await tester.enterText(find.byKey(const Key('notebook-name')), 'Vendor study');
    await tester.enterText(find.byKey(const Key('research-description')), 'Pick a vendor');
    await tester.tap(find.byKey(const Key('create-notebook')));
    await tester.pumpAndSettle();
    expect(state.section, Section.research);
    expect(state.research.first.name, 'Vendor study');
    expect(find.text('Vendor study'), findsWidgets);
    expect(find.byKey(const Key('research-sources')), findsOneWidget);
    expect(find.textContaining('Add a file or folder'), findsOneWidget);
  });

  testWidgets('switching a source to Off updates the context bar', (tester) async {
    await _pump(tester);
    await _openBoard(tester);
    final bar = find.byKey(const Key('research-context-bar'));
    expect(tester.widget<Text>(bar).data, startsWith('2 sources · 1 summaries'));
    await _tab(tester, 'sources');
    await tester.tap(find.byKey(const Key('level-rs2-off')));
    await tester.pumpAndSettle();
    await _tab(tester, 'chat');
    expect(tester.widget<Text>(bar).data, '1 sources · 1 summaries · ≈ 0k tokens');
    await _tab(tester, 'sources');
    await tester.tap(find.byKey(const Key('level-rs1-full')));
    await tester.pumpAndSettle();
    await _tab(tester, 'chat');
    expect(tester.widget<Text>(bar).data, startsWith('1 sources · 0 summaries'));
  });

  testWidgets('an answer cites its source, opens it and saves to a note', (tester) async {
    await _pump(tester);
    await _openBoard(tester);
    await tester.enterText(find.byKey(const Key('research-input')), 'When do we ship?');
    await tester.tap(find.byKey(const Key('research-send')));
    await tester.pumpAndSettle();
    expect(find.textContaining('ship in March [1]'), findsOneWidget);
    expect(find.text('REFERENCES'), findsOneWidget);
    await tester.tap(find.byKey(const Key('cite-1')));
    await tester.pumpAndSettle();
    expect(find.textContaining('a sample summary'), findsOneWidget);
    await tester.tap(find.text('Close'));
    await tester.pumpAndSettle();
    final save = find.byKey(const Key('save-note-ra2'));
    expect(save, findsOneWidget);
    await tester.tap(save);
    await tester.pumpAndSettle();
    await _tab(tester, 'notes');
    expect(find.text('AI'), findsOneWidget);
    expect(find.text('When do we ship?'), findsWidgets);
  });

  testWidgets('a note can be left out of the context', (tester) async {
    final state = await _pump(tester);
    await _openBoard(tester);
    expect(state.researchState!.context.notes, 1);
    await _tab(tester, 'notes');
    await tester.tap(find.byKey(const Key('note-include-rn1')));
    await tester.pumpAndSettle();
    expect(state.researchState!.context.notes, 0);
    expect(state.researchState!.notes.single.include, isFalse);
  });

  testWidgets('sources and notes can be added and removed', (tester) async {
    final state = await _pump(tester);
    await _openBoard(tester);
    await _tab(tester, 'sources');
    await tester.tap(find.byKey(const Key('research-add-source')));
    await tester.pumpAndSettle();
    await tester.enterText(find.byType(TextField).last, '/data/report.md');
    await tester.tap(find.text('OK'));
    await tester.pumpAndSettle();
    expect(find.text('report.md'), findsOneWidget);
    expect(state.research.first.sources, hasLength(3));
    final id = state.researchState!.notebook!.sources.last.id;
    await tester.tap(find.byKey(Key('source-menu-$id')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Remove'));
    await tester.pumpAndSettle();
    expect(find.text('report.md'), findsNothing);

    await _tab(tester, 'notes');
    await tester.tap(find.byKey(const Key('research-add-note')));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const Key('note-text')), 'Check the vendor contract');
    await tester.tap(find.byKey(const Key('note-save')));
    await tester.pumpAndSettle();
    expect(find.text('Check the vendor contract'), findsWidgets);
    await tester.tap(find.byKey(const Key('note-delete-rn1')));
    await tester.pumpAndSettle();
    expect(find.text('Audit follow-up'), findsNothing);

    await _tab(tester, 'chat');
    await tester.tap(find.byKey(const Key('research-new-chat')));
    await tester.pumpAndSettle();
    expect(state.researchState!.chats, hasLength(1));
  });

  testWidgets('a research notebook opens on Ask with tabs for sources and notes', (tester) async {
    await _pump(tester, width: 1000);
    await _openBoard(tester);
    expect(find.byKey(const Key('tab-sources')), findsOneWidget);
    expect(find.byKey(const Key('research-chat')), findsOneWidget);
    await tester.tap(find.byKey(const Key('tab-notes')));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('research-notes')), findsOneWidget);
    expect(find.text('Audit follow-up'), findsOneWidget);
  });

  testWidgets('job notebooks still open the job page', (tester) async {
    final state = await _pump(tester);
    await _openBoard(tester);
    await tester.tap(find.byKey(const Key('activity-notebooks')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Weekly ops report'));
    await tester.pumpAndSettle();
    expect(state.section, Section.notebooks);
    expect(find.byKey(const Key('research-sources')), findsNothing);
  });

  testWidgets('a failed summary event becomes a notice, others reload quietly', (tester) async {
    final core = FakeCoreClient(tick: Duration.zero);
    final state = await _pump(tester, core: core);
    state.listenForEvents();
    await _openBoard(tester);
    core.emit(
      const CoreEvent(kind: 'research.summary.ready', notebookName: 'Board papers', notebookId: 'board-papers'),
    );
    await tester.pumpAndSettle();
    expect(state.notices, isEmpty);
    core.emit(const CoreEvent(kind: 'research.summary.failed', notebookName: 'Board papers', error: 'model offline'));
    await tester.pumpAndSettle();
    expect(state.notices.single, 'Board papers: a summary failed (model offline)');
    state.stopEvents();
  });

  test('fileSection cuts one file out of a folder text', () {
    const text = '== a.go\npackage a\n\n== b/c.go\npackage c\nfunc C() {}\n\n';
    expect(fileSection(text, 'b/c.go'), 'package c\nfunc C() {}');
    expect(fileSection(text, 'a.go'), 'package a');
    expect(fileSection(text, 'missing.go'), text);
    expect(fileSection('plain', null), 'plain');
  });
}
