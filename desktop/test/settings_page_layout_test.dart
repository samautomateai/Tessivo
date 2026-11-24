import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:tessivo_desktop/src/app_state.dart';
import 'package:tessivo_desktop/src/design/components/table.dart';

import 'app_test.dart' show pumpApp;
import 'support/settings_sections.dart';

void main() {
  testWidgets('the side bar lists the sections and each shows its content', (tester) async {
    final state = await pumpApp(tester);
    state.openSettings();
    await tester.pumpAndSettle();
    final content = {
      'models': 'add-provider',
      'defaults': 'defaults-editor',
      'agent': 'agent-editor',
      'mcp': 'mcp-add',
      'appearance': 'appearance-theme',
      'desktop': 'close-to-tray',
      'diagnostics': 'save-diagnostics',
    };
    for (final MapEntry(key: section, value: key) in content.entries) {
      await openSettingsSection(tester, section);
      expect(state.settingsSection.name, section);
      expect(find.byKey(Key(key)), findsOneWidget, reason: section);
      expect(tester.takeException(), isNull, reason: section);
    }
  });

  testWidgets('aliases are a data table with Test and Edit…', (tester) async {
    final state = await pumpApp(tester);
    state.openSettings(SettingsSection.models);
    await tester.pumpAndSettle();
    final table = find.byKey(const Key('aliases-table'));
    expect(tester.widget(table), isA<TvDataTable>());
    expect(find.descendant(of: table, matching: find.text('smart')), findsOneWidget);
    expect(find.descendant(of: table, matching: find.byKey(const Key('test-alias-smart'))), findsOneWidget);
    expect(find.descendant(of: table, matching: find.byKey(const Key('edit-alias-smart'))), findsOneWidget);
    expect(find.descendant(of: table, matching: find.text('Edit…')), findsWidgets);
  });

  testWidgets('Connect a model opens Settings at Models', (tester) async {
    final state = await pumpApp(tester);
    state.openSettings(SettingsSection.desktop);
    state.go(Section.notebooks);
    state.openSettings(SettingsSection.models);
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('add-provider')), findsOneWidget);
  });
}
