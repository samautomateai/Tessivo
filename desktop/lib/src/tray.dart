import 'dart:async';

/// The OS window and tray icon, as the app uses them. The real one is
/// `PluginDesktopWindow` (lib/src/tray_plugins.dart); tests use a fake.
abstract interface class DesktopWindow {
  /// Shows the tray icon with Open and Quit, and routes the window's close
  /// button to [onClose] instead of closing.
  Future<void> init({
    required Future<void> Function() onClose,
    required Future<void> Function() onOpen,
    required Future<void> Function() onQuit,
  });

  Future<void> hide();
  Future<void> show();

  /// Removes the tray icon and closes the window, which ends the app (the
  /// core it started stops with it).
  Future<void> quit();
}

/// Decides what closing the window does: hide it to the tray when the
/// `desktop.close_to_tray` setting is on (runs and schedules go on), else quit.
class TrayController {
  TrayController({required this.window, required this.closeToTray});

  final DesktopWindow window;

  /// Reads the setting when the window is closed, so a change applies at once.
  /// An error counts as off: the app quits as a normal window would.
  final Future<bool> Function() closeToTray;

  Future<void> start() => window.init(onClose: onClose, onOpen: window.show, onQuit: window.quit);

  Future<void> onClose() async {
    var keep = false;
    try {
      keep = await closeToTray();
    } on Object {
      keep = false;
    }
    await (keep ? window.hide() : window.quit());
  }
}
