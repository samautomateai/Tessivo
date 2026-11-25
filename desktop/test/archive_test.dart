import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:tessivo_desktop/src/app_state.dart';

import 'app_test.dart' show pumpApp;
import 'chat_test.dart' show answer;
import 'support/fake_core/fake_core.dart';

void main() {
  testWidgets('Export… writes the archive the core makes to the chosen path', (tester) async {
    final state = await pumpApp(tester);
    final path = '${Directory.systemTemp.createTempSync('tess-export').path}${Platform.pathSeparator}ops.tessivo.zip';
    await tester.tap(find.byKey(const Key('notebook-menu')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('notebook-export')));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const Key('export-path')), path);
    await tester.tap(find.byKey(const Key('export-outputs')));
    await tester.tap(find.byKey(const Key('export-save')));
    await tester.runAsync(() => Future<void>.delayed(const Duration(milliseconds: 50)));
    await tester.pumpAndSettle();
    expect(File(path).readAsStringSync(), startsWith('PK'));
    expect((state.core as FakeCoreClient).exports.last, (state.selectedId!, true));
    expect(find.textContaining('Saved'), findsOneWidget);
  });

  testWidgets('Import… opens the new notebook and asks to relink its sources', (tester) async {
    final state = await pumpApp(tester);
    final file = File('${Directory.systemTemp.createTempSync('tess-import').path}${Platform.pathSeparator}a.zip')
      ..writeAsStringSync('PK fake');
    await tester.tap(find.byKey(const Key('import-notebook')));
    await tester.pumpAndSettle();
    await answer(tester, file.path);
    await tester.runAsync(() => Future<void>.delayed(const Duration(milliseconds: 50)));
    await tester.pumpAndSettle();
    expect(state.selected!.name, endsWith('(imported)'));
    final notice = find.byKey(const Key('relink-notice'));
    expect(notice, findsOneWidget);
    expect(find.descendant(of: notice, matching: find.textContaining('1 source needs relinking')), findsOneWidget);
    final relinkId = state.selected!.context.firstWhere((c) => c.needsRelink).id;
    await tester.tap(find.byKey(Key('relink-$relinkId')));
    await tester.pumpAndSettle();
    await answer(tester, Directory.systemTemp.path);
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('relink-notice')), findsNothing);
  });

  testWidgets('the research side bar imports research archives', (tester) async {
    final state = await pumpApp(tester);
    state.go(Section.research);
    await tester.pumpAndSettle();
    final before = state.research.length;
    final file = File('${Directory.systemTemp.createTempSync('tess-import').path}${Platform.pathSeparator}r.zip')
      ..writeAsStringSync('PK fake');
    await tester.tap(find.byKey(const Key('import-research')));
    await tester.pumpAndSettle();
    await answer(tester, file.path);
    await tester.pumpAndSettle();
    expect(state.research.length, before + 1);
  });
}
