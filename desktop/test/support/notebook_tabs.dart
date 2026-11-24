import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';

/// Opens a notebook page tab: output, sources, prompt, memory or agent.
Future<void> openTab(WidgetTester tester, String tab) async {
  await tester.tap(find.byKey(Key('tab-$tab')));
  await tester.pumpAndSettle();
}
