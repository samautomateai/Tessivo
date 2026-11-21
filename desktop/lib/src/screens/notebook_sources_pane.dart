import 'package:material_ui/material_ui.dart';

import '../app_state.dart';
import '../core/models.dart';
import '../widgets/memory_card.dart';
import '../widgets/text_prompt_dialog.dart';
import '../widgets/work_type_badge.dart';

/// Left pane of a notebook: its name, sources and pinned memory.
class NotebookSourcesPane extends StatelessWidget {
  const NotebookSourcesPane({super.key, required this.state, required this.notebook});

  final AppState state;
  final Notebook notebook;

  @override
  Widget build(BuildContext context) {
    return Container(
      key: const Key('sources-pane'),
      width: 280,
      decoration: BoxDecoration(
        border: Border(right: BorderSide(color: Theme.of(context).dividerColor)),
      ),
      child: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          NotebookHeader(notebook: notebook),
          const SizedBox(height: 16),
          ContextCard(notebook: notebook, onToggle: state.toggleContext, onAdd: state.addContext),
          const SizedBox(height: 16),
          MemoryCard(notebook: notebook, onChanged: state.setMemory),
        ],
      ),
    );
  }
}

class NotebookHeader extends StatelessWidget {
  const NotebookHeader({super.key, required this.notebook});

  final Notebook notebook;

  @override
  Widget build(BuildContext context) {
    return Wrap(
      spacing: 12,
      runSpacing: 6,
      crossAxisAlignment: WrapCrossAlignment.center,
      children: [
        Text(notebook.name, style: Theme.of(context).textTheme.titleLarge?.copyWith(fontWeight: FontWeight.w600)),
        WorkTypeBadge(workType: notebook.workType),
      ],
    );
  }
}

/// The notebook's sources, with toggles and Add.
class ContextCard extends StatelessWidget {
  const ContextCard({super.key, required this.notebook, required this.onToggle, required this.onAdd});

  final Notebook notebook;
  final Future<void> Function(String id, bool enabled) onToggle;
  final Future<void> Function(String path) onAdd;

  Future<void> _add(BuildContext context) async {
    final messenger = ScaffoldMessenger.of(context);
    final path = await askText(
      context,
      title: 'Add context',
      label: 'Absolute path to a file or folder',
      maxLength: 4096,
    );
    if (path == null) return;
    try {
      await onAdd(path);
    } on Object catch (e) {
      messenger.showSnackBar(SnackBar(content: Text(e is FormatException ? e.message : '$e')));
    }
  }

  @override
  Widget build(BuildContext context) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Text('Sources', style: Theme.of(context).textTheme.titleSmall),
                const Spacer(),
                TextButton.icon(
                  key: const Key('context-add'),
                  onPressed: () => _add(context),
                  icon: const Icon(Icons.add, size: 18),
                  label: const Text('Add'),
                ),
              ],
            ),
            if (notebook.context.isEmpty) const Text('No context yet. Add files, folders, code bases or MCP servers.'),
            for (final c in notebook.context)
              CheckboxListTile(
                key: Key('ctx-${c.id}'),
                dense: true,
                contentPadding: EdgeInsets.zero,
                value: c.enabled,
                onChanged: (v) => onToggle(c.id, v ?? false),
                title: Text(c.label),
                subtitle: Text('${c.kind} · ${c.estimatedTokens ~/ 1000}k tok'),
                controlAffinity: ListTileControlAffinity.leading,
              ),
          ],
        ),
      ),
    );
  }
}
