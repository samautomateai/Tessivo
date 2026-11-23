import 'package:material_ui/material_ui.dart';

import '../core/core_client.dart';
import '../theme.dart';

/// Shows whether each model behind an alias answers one tiny request.
Future<void> showModelCheck(BuildContext context, String model, Future<List<ModelCheck>> Function(String) test) =>
    showDialog<void>(
      context: context,
      builder: (_) => ModelCheckDialog(model: model, checks: test(model)),
    );

class ModelCheckDialog extends StatelessWidget {
  const ModelCheckDialog({super.key, required this.model, required this.checks});

  final String model;
  final Future<List<ModelCheck>> checks;

  @override
  Widget build(BuildContext context) => AlertDialog(
    title: Text('Test "$model"'),
    content: SizedBox(
      width: 520,
      child: FutureBuilder<List<ModelCheck>>(
        future: checks,
        builder: (context, snap) {
          if (snap.hasError) {
            final e = snap.error;
            return Text(e is CoreException ? e.message : '$e', key: const Key('model-check-error'));
          }
          if (!snap.hasData) {
            return const Row(
              children: [
                SizedBox(width: 18, height: 18, child: CircularProgressIndicator(strokeWidth: 2)),
                SizedBox(width: 12),
                Text('Sending one short request to each model…'),
              ],
            );
          }
          return Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              for (final c in snap.data!)
                ListTile(
                  key: Key('model-check-${c.model}'),
                  dense: true,
                  contentPadding: EdgeInsets.zero,
                  leading: Icon(
                    c.ok ? Icons.check_circle_outline : Icons.error_outline,
                    color: c.ok ? Colors.green : Theme.of(context).colorScheme.error,
                    semanticLabel: c.ok ? 'answers' : 'failed',
                  ),
                  title: Text(c.model),
                  subtitle: Text(
                    c.ok ? 'Answers in ${c.millis} ms' : c.error,
                    style: const TextStyle(color: Tokens.textMuted),
                  ),
                ),
            ],
          );
        },
      ),
    ),
    actions: [TextButton(onPressed: () => Navigator.of(context).pop(), child: const Text('Close'))],
  );
}
