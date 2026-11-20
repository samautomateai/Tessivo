import 'dart:async';

import 'package:material_ui/material_ui.dart';

import 'src/app_state.dart';
import 'src/core/core_client.dart';
import 'src/core/http_core_client.dart';
import 'src/core/supervisor.dart';
import 'src/shell.dart';
import 'src/theme.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  final supervisor = CoreSupervisor(executable: findTessExecutable());
  // The core is started with --exit-with-parent, so it also stops if the app
  // crashes or is killed.
  final state = AppState(await connectCore(supervisor.ensureRunning));
  unawaited(state.load());
  runApp(TessivoApp(state: state));
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
