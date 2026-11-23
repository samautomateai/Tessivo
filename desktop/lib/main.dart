import 'dart:async';

import 'package:material_ui/material_ui.dart';

import 'src/app_state.dart';
import 'src/core/core_client.dart';
import 'src/core/http_core_client.dart';
import 'src/core/settings_draft.dart';
import 'src/core/supervisor.dart';
import 'src/shell.dart';
import 'src/theme.dart';
import 'src/tray.dart';
import 'src/tray_plugins.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  final supervisor = CoreSupervisor(executable: findTessExecutable());
  // The core is started with --exit-with-parent, so it also stops if the app
  // crashes or is killed.
  final state = AppState(await connectCore(supervisor.ensureRunning));
  unawaited(state.load());
  state.listenForEvents();
  runApp(TessivoApp(state: state));
  unawaited(startTray(TrayController(window: PluginDesktopWindow(), closeToTray: () => closeToTray(state.core))));
}

/// Whether the config asks to keep running in the tray when the window closes.
Future<bool> closeToTray(CoreClient core) async => SettingsDraft.fromJson((await core.getSettings()).raw).closeToTray;

/// Starts the tray; without one (no tray on this desktop) the window closes as usual.
Future<void> startTray(TrayController tray) async {
  try {
    await tray.start();
  } on Object catch (e) {
    debugPrint('Tessivo tray not available ($e)');
  }
}

/// Connects to the core (reusing or starting `tess serve`); falls back to
/// demo data when it cannot.
Future<CoreClient> connectCore(Future<Endpoint?> Function() connect) async {
  try {
    final ep = await connect();
    if (ep != null) return HttpCoreClient(ep);
  } on Object catch (e) {
    debugPrint('Tessivo core not available ($e); using demo data');
  }
  return DemoCoreClient();
}

class TessivoApp extends StatelessWidget {
  const TessivoApp({super.key, required this.state});

  final AppState state;

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'Tessivo',
      debugShowCheckedModeBanner: false,
      theme: buildTheme(Brightness.light),
      darkTheme: buildTheme(Brightness.dark),
      home: AppShell(state: state),
    );
  }
}
