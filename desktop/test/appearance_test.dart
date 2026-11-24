import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:material_ui/material_ui.dart';
import 'package:tessivo_desktop/src/app_state.dart';
import 'package:tessivo_desktop/src/core/settings_draft.dart';
import 'package:tessivo_desktop/src/design/colors.dart';
import 'package:tessivo_desktop/src/design/theme.dart';
import 'package:tessivo_desktop/src/design/zoom.dart';
import 'package:tessivo_desktop/src/shell/workbench.dart';

import 'app_test.dart' show pumpApp;
import 'support/fake_core/fake_core.dart';
import 'support/settings_sections.dart';

Future<Map<String, Object?>> desktopBlock(AppState state) async =>
    SettingsDraft.fromJson((await state.core.getSettings()).raw).desktop;

void main() {
  test('nextZoom steps by 0.1 within 0.8–2.0 and resets to 1.0', () {
    expect(nextZoom(1.0, 1), 1.1);
    expect(nextZoom(1.9, 1), 2.0);
    expect(nextZoom(2.0, 1), 2.0);
    expect(nextZoom(0.8, -1), 0.8);
    expect(nextZoom(1.0, -1), 0.9);
    expect(nextZoom(1.7, 0), 1.0);
  });

  testWidgets('Ctrl = / Ctrl - / Ctrl 0 zoom and remember it', (tester) async {
    final state = await pumpApp(tester);
    Future<void> press(LogicalKeyboardKey key) async {
      await tester.sendKeyDownEvent(LogicalKeyboardKey.controlLeft);
      await tester.sendKeyEvent(key);
      await tester.sendKeyUpEvent(LogicalKeyboardKey.controlLeft);
      await tester.pumpAndSettle();
    }

    await press(LogicalKeyboardKey.equal);
    await press(LogicalKeyboardKey.equal);
    expect(state.zoom, 1.2);
    expect((await desktopBlock(state))['zoom'], 1.2);
    await press(LogicalKeyboardKey.minus);
    expect(state.zoom, 1.1);
    await press(LogicalKeyboardKey.digit0);
    expect(state.zoom, 1.0);
    expect((await desktopBlock(state)).containsKey('zoom'), isFalse);
  });

  testWidgets('zoom scales the whole UI: half the logical size at 2.0', (tester) async {
    final state = await pumpApp(tester);
    await state.setZoom(2.0);
    await tester.pumpAndSettle();
    final size = MediaQuery.sizeOf(tester.element(find.byType(Workbench)));
    expect(size, const Size(800, 500)); // the test window is 1600×1000
    expect(TvZoom.of(tester.element(find.byType(Workbench))).factor, 2.0);
  });

  testWidgets('the theme menu switches the tokens at once', (tester) async {
    final state = await pumpApp(tester);
    TvColors colors() => TvColors.of(tester.element(find.byType(Workbench)));
    expect(colors(), TvColors.light); // the test window reports a light OS
    await state.setThemeMode(TvThemeMode.dark);
    await tester.pumpAndSettle();
    expect(colors(), TvColors.dark);
    expect((await desktopBlock(state))['theme'], 'dark');
    await tester.tap(find.text('Settings'));
    await tester.pumpAndSettle();
    await openSettingsSection(tester, 'appearance');
    await tester.scrollUntilVisible(
      find.byKey(const Key('appearance-theme')),
      200,
      scrollable: find.descendant(of: find.byKey(const Key('settings-list')), matching: find.byType(Scrollable)).first,
    );
    await tester.tap(find.byKey(const Key('appearance-theme')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('High contrast').last);
    await tester.pumpAndSettle();
    expect(colors(), TvColors.highContrast);
    expect(find.textContaining('display 1.00 × text 1.00'), findsOneWidget);
  });

  testWidgets('a hand-edited config with bad values loads the defaults', (tester) async {
    final core = FakeCoreClient(tick: Duration.zero);
    await core.updateSettings({
      'desktop': {'theme': 'purple', 'zoom': 9},
    });
    final state = AppState(core);
    await state.load();
    expect((state.themeMode, state.zoom), (TvThemeMode.system, 1.0));
  });

  test('settings draft reads and writes theme and zoom', () {
    final d = SettingsDraft.fromJson({
      'desktop': {'theme': 'high-contrast', 'zoom': 1.5},
    });
    expect((d.theme, d.zoom), (TvThemeMode.highContrast, 1.5));
    d
      ..theme = TvThemeMode.system
      ..zoom = 1.0;
    expect(d.desktop.containsKey('theme') || d.desktop.containsKey('zoom'), isFalse);
    d.theme = TvThemeMode.light;
    expect(d.desktop['theme'], 'light');
  });
}
