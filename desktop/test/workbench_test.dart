import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:tessivo_desktop/src/app_state.dart';
import 'package:tessivo_desktop/src/core/settings_draft.dart';
import 'package:tessivo_desktop/src/shell/status_bar.dart';
import 'package:tessivo_desktop/src/shell/workbench.dart';

import 'app_test.dart' show pumpApp;

Future<Map<String, Object?>> layoutOf(AppState state) async =>
    SettingsDraft.fromJson((await state.core.getSettings()).raw).layout;

Future<void> ctrl(WidgetTester tester, LogicalKeyboardKey key) async {
  await tester.sendKeyDownEvent(LogicalKeyboardKey.controlLeft);
  await tester.sendKeyEvent(key);
  await tester.sendKeyUpEvent(LogicalKeyboardKey.controlLeft);
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('header shows breadcrumbs, search and a palette hint', (tester) async {
    final state = await pumpApp(tester);
    expect(find.byType(Workbench), findsOneWidget);
    expect(find.byKey(const Key('breadcrumbs')), findsOneWidget);
    expect(find.text('Notebooks / ${state.selected!.name}'), findsOneWidget);
    expect(find.byKey(const Key('sidebar-search')), findsOneWidget);
    await tester.tap(find.byKey(const Key('palette-hint')));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('palette-input')), findsOneWidget);
  });

  testWidgets('the activity bar shows labelled sections and switches them', (tester) async {
    final state = await pumpApp(tester);
    for (final label in ['Notebooks', 'Research', 'Chat', 'Schedules', 'Templates', 'Outputs', 'Settings']) {
      expect(find.descendant(of: find.byKey(const Key('activity-bar')), matching: find.text(label)), findsOneWidget);
    }
    await tester.tap(find.descendant(of: find.byKey(const Key('activity-bar')), matching: find.text('Templates')));
    await tester.pumpAndSettle();
    expect(state.section, Section.templates);
  });

  testWidgets('Ctrl B hides the side bar and remembers it', (tester) async {
    final state = await pumpApp(tester);
    expect(find.byKey(const Key('side-bar')), findsOneWidget);
    await ctrl(tester, LogicalKeyboardKey.keyB);
    expect(find.byKey(const Key('side-bar')), findsNothing);
    expect((await layoutOf(state))['side_bar_open'], false);
    await ctrl(tester, LogicalKeyboardKey.keyB);
    expect(find.byKey(const Key('side-bar')), findsOneWidget);
  });

  testWidgets('dragging the side bar edge resizes it within 180–480 and saves the width', (tester) async {
    final state = await pumpApp(tester);
    await tester.drag(find.byKey(const Key('side-bar-edge')), const Offset(600, 0));
    await tester.pumpAndSettle();
    expect(tester.getSize(find.byKey(const Key('side-bar'))).width, 480);
    expect((await layoutOf(state))['side_bar_width'], 480);
    await tester.drag(find.byKey(const Key('side-bar-edge')), const Offset(-900, 0));
    await tester.pumpAndSettle();
    expect(tester.getSize(find.byKey(const Key('side-bar'))).width, 180);
  });

  testWidgets('narrow windows close the side bar', (tester) async {
    await pumpApp(tester);
    tester.view.physicalSize = const Size(800, 700);
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('side-bar')), findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets('the status bar shows the core, model, tokens and approvals', (tester) async {
    final state = await pumpApp(tester);
    final bar = find.byKey(const Key('status-bar'));
    expect(tester.getSize(bar).height, 22);
    expect(find.descendant(of: bar, matching: find.text('Core connected')), findsOneWidget);
    expect(find.descendant(of: bar, matching: find.textContaining(state.defaultModel!)), findsOneWidget);
    expect(find.descendant(of: bar, matching: find.textContaining('tokens')), findsOneWidget);
    if (state.approvals.isNotEmpty) {
      expect(find.descendant(of: bar, matching: find.textContaining('approval')), findsOneWidget);
    }
  });

  test('token counts read like 412k and 2M', () {
    expect(formatTokens(950), '950');
    expect(formatTokens(412000), '412k');
    expect(formatTokens(2000000), '2M');
    expect(formatTokens(1250000), '1.3M');
    expect(usageText(412000, 2000000), 'Today 412k / 2M tokens');
    expect(usageText(412000, 0), 'Today 412k tokens');
  });

  // The notebook page is rebuilt in Task 10, which checks it at 200 % itself.
  testWidgets('at 200 % zoom on a 1366×768 window nothing overflows', (tester) async {
    final state = await pumpApp(tester);
    state.go(Section.chat);
    await tester.pumpAndSettle();
    tester.view.physicalSize = const Size(1366, 768);
    await state.setZoom(2.0);
    await tester.pumpAndSettle();
    expect(tester.takeException(), isNull);
    for (final s in [Section.chat, Section.settings, Section.templates, Section.schedules]) {
      state.go(s);
      await tester.pumpAndSettle();
      expect(tester.takeException(), isNull, reason: '$s');
    }
  });
}
