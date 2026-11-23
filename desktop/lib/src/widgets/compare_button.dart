import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../core/core_client.dart';
import '../core/models.dart';

/// "Compare": the open revision against the version it revised.
class CompareButton extends StatelessWidget {
  const CompareButton({super.key, required this.core, required this.notebookId, required this.outputId});

  final CoreClient core;
  final String notebookId;
  final String outputId;

  Future<void> _open(BuildContext context) async {
    final messenger = ScaffoldMessenger.of(context);
    final OutputDiffInfo d;
    try {
      d = await core.diffOutput(notebookId, outputId);
    } on CoreException catch (e) {
      messenger.showSnackBar(SnackBar(content: Text(e.message)));
      return;
    }
    if (!context.mounted) return;
    await showDialog<void>(
      context: context,
      builder: (_) => _DiffDialog(diff: d),
    );
  }

  @override
  Widget build(BuildContext context) => TextButton.icon(
    key: const Key('compare'),
    onPressed: () => unawaited(_open(context)),
    icon: const Icon(Icons.difference_outlined, size: 18),
    label: const Text('Compare'),
  );
}

class _DiffDialog extends StatelessWidget {
  const _DiffDialog({required this.diff});

  final OutputDiffInfo diff;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    Color? bg(String op) => switch (op) {
      '+' => Colors.green.withValues(alpha: 0.15),
      '-' => scheme.error.withValues(alpha: 0.12),
      _ => null,
    };
    return AlertDialog(
      key: const Key('diff-view'),
      title: Text('Changes: ${diff.added} added, ${diff.removed} removed'),
      content: SizedBox(
        width: 760,
        height: 480,
        child: ListView(
          children: [
            for (final l in diff.lines)
              Semantics(
                label: switch (l.op) {
                  '+' => 'Added: ${l.text}',
                  '-' => 'Removed: ${l.text}',
                  _ => l.text,
                },
                child: Container(
                  color: bg(l.op),
                  padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 1),
                  child: Text('${l.op} ${l.text}', style: const TextStyle(fontFamily: 'monospace', fontSize: 13)),
                ),
              ),
          ],
        ),
      ),
      actions: [TextButton(onPressed: () => Navigator.of(context).pop(), child: const Text('Close'))],
    );
  }
}
