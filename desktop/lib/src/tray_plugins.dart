import 'dart:async';
import 'dart:io';

import 'package:tray_manager/tray_manager.dart';
import 'package:window_manager/window_manager.dart';

import 'tray.dart';

/// [DesktopWindow] over the tray_manager and window_manager plugins (ADR 0007).
/// Only `main.dart` creates it; widget tests have no plugin channels.
class PluginDesktopWindow with WindowListener, TrayListener implements DesktopWindow {
  Future<void> Function() _onClose = () async {};
  Future<void> Function() _onOpen = () async {};
  Future<void> Function() _onQuit = () async {};

  @override
  Future<void> init({
    required Future<void> Function() onClose,
    required Future<void> Function() onOpen,
    required Future<void> Function() onQuit,
  }) async {
    _onClose = onClose;
    _onOpen = onOpen;
    _onQuit = onQuit;
    await windowManager.ensureInitialized();
    // Windows tray icons must be .ico; macOS and Linux take a PNG (approved, ADR 0007).
    await trayManager.setIcon(Platform.isWindows ? 'assets/tray/tray.ico' : 'assets/tray/tray.png');
    await trayManager.setToolTip('Tessivo');
    await trayManager.setContextMenu(
      Menu(
        items: [
          MenuItem(key: 'open', label: 'Open Tessivo'),
          MenuItem.separator(),
          MenuItem(key: 'quit', label: 'Quit Tessivo'),
        ],
      ),
    );
    trayManager.addListener(this);
    // Last, so a tray that fails to appear leaves the close button working.
    await windowManager.setPreventClose(true);
    windowManager.addListener(this);
  }

  @override
  Future<void> hide() => windowManager.hide();

  @override
  Future<void> show() async {
    await windowManager.show();
    await windowManager.focus();
  }

  @override
  Future<void> quit() async {
    trayManager.removeListener(this);
    windowManager.removeListener(this);
    await trayManager.destroy();
    await windowManager.setPreventClose(false);
    await windowManager.destroy();
  }

  @override
  void onWindowClose() => unawaited(_onClose());

  @override
  void onTrayIconMouseDown() => unawaited(_onOpen());

  @override
  void onTrayIconRightMouseDown() => unawaited(trayManager.popUpContextMenu());

  @override
  void onTrayMenuItemClick(MenuItem menuItem) => unawaited(menuItem.key == 'quit' ? _onQuit() : _onOpen());
}
