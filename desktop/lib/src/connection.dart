import 'dart:async';

import 'package:material_ui/material_ui.dart';

import 'core/core_client.dart';
import 'core/supervisor.dart';
import 'screens/connection_screen.dart';

/// Where the app is with its core.
sealed class CoreStatus {
  const CoreStatus();
}

/// The supervisor is starting (or finding) `tess serve`.
final class CoreStarting extends CoreStatus {
  const CoreStarting();
}

/// The core could not be reached; [reason] is one line, [details] everything
/// worth copying into a bug report (already redacted by the supervisor).
final class CoreFailed extends CoreStatus {
  const CoreFailed(this.reason, this.details);
  final String reason;
  final String details;
}

/// Connected; the app runs on [client].
final class CoreConnected extends CoreStatus {
  const CoreConnected(this.client);
  final CoreClient client;
}

/// Back-off between reconnect attempts: 1, 2, 4, 8, then 15 seconds.
Duration reconnectDelay(int attempt) => Duration(seconds: attempt >= 4 ? 15 : 1 << attempt);

/// Connects to the core and shows the connection screen until it answers;
/// then shows [builder]'s widget. It never shows sample data.
class ConnectionGate extends StatefulWidget {
  const ConnectionGate({super.key, required this.connect, required this.builder});

  final Future<CoreClient> Function() connect;
  final Widget Function(CoreClient client) builder;

  @override
  State<ConnectionGate> createState() => _ConnectionGateState();
}

class _ConnectionGateState extends State<ConnectionGate> {
  CoreStatus _status = const CoreStarting();

  @override
  void initState() {
    super.initState();
    unawaited(_connect());
  }

  Future<void> _connect() async {
    setState(() {
      _status = const CoreStarting();
    });
    CoreStatus next;
    try {
      next = CoreConnected(await widget.connect());
    } on Object catch (e) {
      final text = e is CoreStartException ? e.message : '$e';
      next = CoreFailed(text.split('\n').first, text);
    }
    if (mounted) {
      setState(() {
        _status = next;
      });
    }
  }

  @override
  Widget build(BuildContext context) => switch (_status) {
    CoreConnected(:final client) => widget.builder(client),
    final s => MaterialApp(
      title: 'Tessivo',
      debugShowCheckedModeBanner: false,
      home: ConnectionScreen(status: s, onRetry: () => unawaited(_connect())),
    ),
  };
}
