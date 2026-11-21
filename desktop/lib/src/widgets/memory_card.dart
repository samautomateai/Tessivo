import 'package:material_ui/material_ui.dart';

import '../core/models.dart';
import '../theme.dart';
import 'text_prompt_dialog.dart';

/// Notebook memory: short facts sent with every run of this notebook.
class MemoryCard extends StatelessWidget {
  const MemoryCard({super.key, required this.notebook, required this.onChanged});

  final Notebook notebook;
  final Future<void> Function(List<String> memory) onChanged;

  Future<void> _add(BuildContext context) async {
    final fact = await askText(
      context,
      title: 'Remember for this notebook',
      label: 'Fact',
      maxLength: maxMemoryItemLength,
    );
    if (fact != null && context.mounted) await _save(context, [...notebook.memory, fact]);
  }

  Future<void> _save(BuildContext context, List<String> memory) async {
    final messenger = ScaffoldMessenger.of(context);
    try {
      await onChanged(memory);
    } on Object catch (e) {
      messenger.showSnackBar(SnackBar(content: Text(e is FormatException ? e.message : '$e')));
    }
  }

  @override
  Widget build(BuildContext context) {
    final memory = notebook.memory;
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Text('Memory', style: Theme.of(context).textTheme.titleSmall),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    '${memory.length}/$maxMemoryItems',
                    style: const TextStyle(color: Tokens.textMuted, fontSize: 12),
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
                TextButton.icon(
                  key: const Key('memory-add'),
                  onPressed: memory.length >= maxMemoryItems ? null : () => _add(context),
                  icon: const Icon(Icons.add, size: 18),
                  label: const Text('Add'),
                ),
              ],
            ),
            if (memory.isEmpty)
              const Text(
                'Facts the model should always know here, like naming rules or the audience.',
                style: TextStyle(color: Tokens.textMuted),
              ),
            for (final (i, fact) in memory.indexed)
              ListTile(
                key: Key('memory-$i'),
                dense: true,
                contentPadding: EdgeInsets.zero,
                title: Text(fact),
                trailing: IconButton(
                  key: Key('memory-remove-$i'),
                  tooltip: 'Forget',
                  icon: const Icon(Icons.close, size: 18),
                  onPressed: () => _save(context, [...memory]..removeAt(i)),
                ),
              ),
          ],
        ),
      ),
    );
  }
}
