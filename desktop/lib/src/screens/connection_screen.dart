import 'package:flutter/services.dart';
import 'package:material_ui/material_ui.dart';

import '../connection.dart';

/// Full-window state while the app has no core: starting, or why it could
/// not be reached and what to do about it.
class ConnectionScreen extends StatelessWidget {
  const ConnectionScreen({super.key, required this.status, required this.onRetry});

  final CoreStatus status;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    final text = Theme.of(context).textTheme;
    final s = status;
    return Scaffold(
      body: Center(
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 560),
          child: Padding(
            padding: const EdgeInsets.all(32),
            child: s is CoreFailed
                ? Column(
                    mainAxisSize: MainAxisSize.min,
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text("Can't reach the Tessivo core", style: text.titleMedium),
                      const SizedBox(height: 8),
                      SelectableText(s.reason, key: const Key('connection-reason')),
                      const SizedBox(height: 16),
                      const Text(
                        'The app runs its work through tess, which normally starts by itself. Check that tess '
                        'is installed next to the app, or start it yourself in a terminal with: tess serve',
                      ),
                      const SizedBox(height: 24),
                      Row(
                        children: [
                          FilledButton(
                            key: const Key('connection-retry'),
                            onPressed: onRetry,
                            child: const Text('Retry'),
                          ),
                          const SizedBox(width: 8),
                          TextButton(
                            key: const Key('connection-copy'),
                            onPressed: () => Clipboard.setData(ClipboardData(text: s.details)),
                            child: const Text('Copy details'),
                          ),
                        ],
                      ),
                    ],
                  )
                : const Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2)),
                      SizedBox(width: 12),
                      Text('Starting the Tessivo core…'),
                    ],
                  ),
          ),
        ),
      ),
    );
  }
}
