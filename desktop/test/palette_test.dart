import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:tessivo_desktop/src/app_state.dart';

import 'app_test.dart' show pumpApp;

Future<void> chord(WidgetTester tester, LogicalKeyboardKey key) async {
  await tester.sendKeyDownEvent(LogicalKeyboardKey.controlLeft);
  await tester.sendKeyEvent(key);
  await tester.sendKeyUpEvent(LogicalKeyboardKey.controlLeft);
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('Ctrl+K opens the palette; typing filters; Enter runs', (tester) async {
    final state = await pumpApp(tester);
    await chord(tester, LogicalKeyboardKey.keyK);
    expect(find.byKey(const Key('palette')), findsOneWidget);
    await tester.enterText(find.byKey(const Key('palette-input')), 'weekly');
    await tester.pumpAndSettle();
    expect(find.text('Open notebook: Weekly ops report'), findsOneWidget);
    expect(find.text('Go to Templates'), findsNothing);
    await tester.testTextInput.receiveAction(TextInputAction.done);
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('palette')), findsNothing);
    expect(state.selected?.name, 'Weekly ops report');

    await chord(tester, LogicalKeyboardKey.keyK);
    await tester.enterText(find.byKey(const Key('palette-input')), 'schedules');
    await tester.pumpAndSettle();
    await tester.tap(find.text('Go to Schedules'));
    await tester.pumpAndSettle();
    expect(state.section, Section.schedules);
  });

  testWidgets('Ctrl+, opens settings and Ctrl+N a new notebook', (tester) async {
    final state = await pumpApp(tester);
    await chord(tester, LogicalKeyboardKey.comma);
    expect(state.section, Section.settings);
    await chord(tester, LogicalKeyboardKey.keyN);
    expect(find.text('What kind of work is this notebook for?'), findsOneWidget);
  });

  testWidgets('the palette searches what you typed', (tester) async {
    await pumpApp(tester);
    await chord(tester, LogicalKeyboardKey.keyK);
    await tester.enterText(find.byKey(const Key('palette-input')), 'risk');
    await tester.pumpAndSettle();
    await tester.tap(find.text('Search for "risk"'));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('search-results')), findsOneWidget);
  });
}
