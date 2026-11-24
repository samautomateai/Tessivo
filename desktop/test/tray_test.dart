import 'package:flutter_test/flutter_test.dart';
import 'package:tessivo_desktop/main.dart';
import 'package:tessivo_desktop/src/core/settings_draft.dart';
import 'package:tessivo_desktop/src/tray.dart';

import 'support/fake_core/fake_core.dart';

class FakeWindow implements DesktopWindow {
  final calls = <String>[];
  Future<void> Function()? close, open, quitHook;

  @override
  Future<void> init({
    required Future<void> Function() onClose,
    required Future<void> Function() onOpen,
    required Future<void> Function() onQuit,
  }) async {
    close = onClose;
    open = onOpen;
    quitHook = onQuit;
    calls.add('init');
  }

  @override
  Future<void> hide() async => calls.add('hide');

  @override
  Future<void> show() async => calls.add('show');

  @override
  Future<void> quit() async => calls.add('quit');
}

void main() {
  mainHelperTests();
  test('closing hides to the tray when the setting is on', () async {
    final w = FakeWindow();
    var on = true;
    final c = TrayController(window: w, closeToTray: () async => on);
    await c.start();
    await w.close!();
    on = false;
    await w.close!();
    expect(w.calls, ['init', 'hide', 'quit']);
  });

  test('the tray menu opens and quits; a settings error quits', () async {
    final w = FakeWindow();
    final c = TrayController(window: w, closeToTray: () async => throw StateError('core gone'));
    await c.start();
    await w.open!();
    await w.quitHook!();
    await c.onClose();
    expect(w.calls, ['init', 'show', 'quit', 'quit']);
  });

  test('settings draft keeps and sets the desktop block', () {
    final d = SettingsDraft.fromJson({
      'providers': <Object?>[],
      'aliases': <Object?>[],
      'desktop': {'close_to_tray': true, 'future_field': 1},
    });
    expect(d.closeToTray, isTrue);
    d.closeToTray = false;
    expect(d.toBody()['desktop'], {'future_field': 1});
    d.closeToTray = true;
    expect((d.toBody()['desktop']! as Map)['close_to_tray'], isTrue);
    expect(SettingsDraft.fromJson({}).closeToTray, isFalse);
  });
}

class BrokenWindow extends FakeWindow {
  @override
  Future<void> init({
    required Future<void> Function() onClose,
    required Future<void> Function() onOpen,
    required Future<void> Function() onQuit,
  }) async => throw StateError('no tray on this desktop');
}

void mainHelperTests() {
  test('closeToTray reads the core settings; startTray survives a missing tray', () async {
    final core = FakeCoreClient(tick: Duration.zero);
    expect(await closeToTray(core), isFalse);
    await core.updateSettings({
      'desktop': {'close_to_tray': true},
    });
    expect(await closeToTray(core), isTrue);
    await startTray(TrayController(window: BrokenWindow(), closeToTray: () async => true));
    final ok = FakeWindow();
    await startTray(TrayController(window: ok, closeToTray: () async => true));
    expect(ok.calls, ['init']);
  });
}
