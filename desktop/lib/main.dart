import 'package:material_ui/material_ui.dart';

import 'src/app_state.dart';
import 'src/core/core_client.dart';
import 'src/shell.dart';
import 'src/theme.dart';

void main() {
  runApp(TessivoApp(state: AppState(DemoCoreClient())..load()));
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
