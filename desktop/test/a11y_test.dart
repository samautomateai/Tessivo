import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';

import 'app_test.dart' show pumpApp;

/// Each main screen must meet Flutter's accessibility guidelines: every tap
/// target has a label, and text contrast is high enough.
Future<void> checkScreen(WidgetTester tester) async {
  await expectLater(tester, meetsGuideline(labeledTapTargetGuideline));
  await expectLater(tester, meetsGuideline(textContrastGuideline));
}

void main() {
  for (final section in ['Notebooks', 'Chat', 'Templates', 'Schedules', 'Outputs', 'Settings']) {
    testWidgets('$section meets the accessibility guidelines', (tester) async {
      final handle = tester.ensureSemantics();
      await pumpApp(tester);
      if (section != 'Notebooks') {
        await tester.tap(find.descendant(of: find.byKey(const Key('activity-bar')), matching: find.text(section)));
        await tester.pumpAndSettle();
      }
      await checkScreen(tester);
      handle.dispose();
    });
  }

  testWidgets('screen readers hear which item is selected', (tester) async {
    final handle = tester.ensureSemantics();
    await pumpApp(tester);
    expect(
      tester.getSemantics(find.byKey(const Key('nav-nb-1'))),
      isSemantics(label: 'Change risk review, Analyze notebook', isSelected: true, isButton: true),
    );
    await tester.tap(find.byKey(const Key('new-notebook')));
    await tester.pumpAndSettle();
    expect(tester.getSemantics(find.byKey(const Key('type-analyze'))), isSemantics(isSelected: true));
    expect(tester.getSemantics(find.byKey(const Key('type-report'))), isSemantics(isSelected: false));
    handle.dispose();
  });

  testWidgets('a research notebook meets the accessibility guidelines', (tester) async {
    final handle = tester.ensureSemantics();
    await pumpApp(tester);
    await tester.tap(find.byKey(const Key('activity-research')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('research-board-papers')));
    await tester.pumpAndSettle();
    await checkScreen(tester);
    handle.dispose();
  });
}
