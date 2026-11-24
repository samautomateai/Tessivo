import 'dart:async';
import 'dart:io';

import 'package:material_ui/material_ui.dart';

import 'src/app_state.dart';
import 'src/connection.dart';
import 'src/core/core_client.dart';
import 'src/core/http_core_client.dart';
import 'src/core/settings_draft.dart';
import 'src/core/supervisor.dart';
import 'src/design/colors.dart';
import 'src/design/theme.dart';
import 'src/shell.dart';
import 'src/tray.dart';
import 'src/tray_plugins.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  final supervisor = CoreSupervisor(executable: findTessExecutable(override: Platform.environment['TESSIVO_TESS']));
  // The core is started with --exit-with-parent, so it also stops if the app
  // crashes or is killed. Until it answers, the gate shows the connection screen.
  runApp(ConnectionGate(connect: () => connectCore(supervisor.ensureRunning), builder: startApp));
}

/// Builds the app on a connected core (once per successful connection).
Widget startApp(CoreClient core) {
  final state = AppState(core);
  unawaited(state.load());
  state.listenForEvents();
  unawaited(startTray(TrayController(window: PluginDesktopWindow(), closeToTray: () => closeToTray(state.core))));
  return TessivoApp(state: state);
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

/// Connects to the core (reusing or starting `tess serve`). It never falls
/// back to sample data: a failure reaches the caller.
Future<CoreClient> connectCore(Future<Endpoint?> Function() connect) async {
  final ep = await connect();
  if (ep == null) throw const CoreStartException('the Tessivo core did not start');
  return HttpCoreClient(ep);
}

class TessivoApp extends StatelessWidget {
  const TessivoApp({super.key, required this.state});

  final AppState state;

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'Tessivo',
      debugShowCheckedModeBanner: false,
      theme: tvTheme(TvColors.light),
      darkTheme: tvTheme(TvColors.dark),
      highContrastTheme: tvTheme(TvColors.highContrast),
      highContrastDarkTheme: tvTheme(TvColors.highContrast),
      home: AppShell(state: state),
    );
  }
}
