import 'dart:async';

import 'package:material_ui/material_ui.dart';

import 'src/app_state.dart';
import 'src/core/core_client.dart';
import 'src/core/http_core_client.dart';
import 'src/shell.dart';
import 'src/theme.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  final state = AppState(await connectCore());
  unawaited(state.load());
  runApp(TessivoApp(state: state));
}

/// Connects to a running core (`tess serve`); falls back to demo data.
Future<CoreClient> connectCore({Future<Endpoint?> Function()? discover}) async {
  try {
    final ep = await (discover ?? Endpoint.discover)();
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
