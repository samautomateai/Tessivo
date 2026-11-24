import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';

/// Opens a Settings section from its side bar: models, defaults, agent, mcp,
/// appearance, desktop or diagnostics.
Future<void> openSettingsSection(WidgetTester tester, String section) async {
  await tester.tap(find.byKey(Key('settings-$section')));
  await tester.pumpAndSettle();
}
