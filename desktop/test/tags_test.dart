import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:tessivo_desktop/src/app_state.dart';
import 'package:tessivo_desktop/src/design/components/headers.dart';
import 'package:tessivo_desktop/src/design/components/tags.dart';

import 'app_test.dart' show pumpApp;

Finder inSideBar(Finder f) => find.descendant(of: find.byKey(const Key('side-bar')), matching: f);

void main() {
  testWidgets('a pinned notebook is listed under PINNED above the rest', (tester) async {
    final state = await pumpApp(tester);
    await state.setPinned('nb-4', true);
    await tester.pumpAndSettle();
    final pinnedLabel = tester.getTopLeft(inSideBar(find.text('PINNED'))).dy;
    final pinned = tester.getTopLeft(find.byKey(const Key('nav-nb-4'))).dy;
    final other = tester.getTopLeft(find.byKey(const Key('nav-nb-1'))).dy;
    expect(pinnedLabel < pinned && pinned < other, isTrue);
    expect(state.notebooks.firstWhere((n) => n.id == 'nb-4').pinned, isTrue);
  });

  testWidgets('Pin and Edit tags… in the notebook menu save through the core', (tester) async {
    final state = await pumpApp(tester);
    await tester.tap(find.byKey(const Key('notebook-menu')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('notebook-pin')));
    await tester.pumpAndSettle();
    expect(state.selected!.pinned, isTrue);

    await tester.tap(find.byKey(const Key('notebook-menu')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('notebook-tags')));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const Key('text-prompt')), 'Infra, Q3 review');
    await tester.tap(find.byKey(const Key('text-prompt-ok')));
    await tester.pumpAndSettle();
    expect(state.selected!.tags, ['Infra', 'Q3 review']);
    final header = find.byType(TvPageHeader);
    expect(find.descendant(of: header, matching: find.widgetWithText(TvTag, 'Infra')), findsOneWidget);
  });

  testWidgets('the tag filter leaves only notebooks with that tag', (tester) async {
    final state = await pumpApp(tester);
    await state.setTags('nb-2', ['Vendors']);
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const Key('tag-filter')), 'vend');
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('nav-nb-2')), findsOneWidget);
    expect(find.byKey(const Key('nav-nb-1')), findsNothing);
    await tester.enterText(find.byKey(const Key('tag-filter')), '');
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('nav-nb-1')), findsOneWidget);
  });

  testWidgets('a bad tag is refused with a notice and many tags do not overflow at 200 %', (tester) async {
    final state = await pumpApp(tester);
    await state.setTags(state.selectedId!, ['a/b']);
    await tester.pumpAndSettle();
    expect(state.selected!.tags, isEmpty);
    expect(state.notices, isNotEmpty);
    await state.setTags(state.selectedId!, [for (var i = 0; i < 10; i++) 'tag number $i']);
    tester.view.physicalSize = const Size(1366, 768);
    await state.setZoom(2.0);
    await tester.pumpAndSettle();
    expect(tester.takeException(), isNull);
  });
}
