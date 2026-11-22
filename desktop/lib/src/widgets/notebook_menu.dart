import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../app_state.dart';
import '../core/models.dart';
import 'text_prompt_dialog.dart';

/// Rename, archive or restore, and delete for the open notebook.
class NotebookMenu extends StatelessWidget {
  const NotebookMenu({super.key, required this.state, required this.notebook});

  final AppState state;
  final Notebook notebook;

  Future<void> _rename(BuildContext context) async {
    final name = await askText(context, title: 'Rename notebook', initial: notebook.name, maxLength: 200);
    if (name != null && name.trim().isNotEmpty) await state.renameNotebook(notebook.id, name.trim());
  }

  Future<void> _delete(BuildContext context) async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('Delete "${notebook.name}"?'),
        content: const Text(
          'This deletes the notebook folder with its sources, prompts, runs and outputs. It cannot be undone.',
        ),
        actions: [
          TextButton(onPressed: () => Navigator.of(ctx).pop(false), child: const Text('Cancel')),
          FilledButton(
            key: const Key('confirm-delete'),
            style: FilledButton.styleFrom(backgroundColor: Theme.of(ctx).colorScheme.error),
            onPressed: () => Navigator.of(ctx).pop(true),
            child: const Text('Delete'),
          ),
        ],
      ),
    );
    if (ok ?? false) await state.deleteNotebook(notebook.id);
  }

  @override
  Widget build(BuildContext context) {
    return PopupMenuButton<String>(
      key: const Key('notebook-menu'),
      tooltip: 'Notebook',
      icon: const Icon(Icons.more_vert),
      onSelected: (v) => unawaited(switch (v) {
        'rename' => _rename(context),
        'archive' => state.setArchived(notebook.id, true),
        'restore' => state.setArchived(notebook.id, false),
        _ => _delete(context),
      }),
      itemBuilder: (_) => [
        const PopupMenuItem(key: Key('notebook-rename'), value: 'rename', child: Text('Rename…')),
        if (notebook.archived)
          const PopupMenuItem(key: Key('notebook-restore'), value: 'restore', child: Text('Restore from archive'))
        else
          const PopupMenuItem(key: Key('notebook-archive'), value: 'archive', child: Text('Archive')),
        const PopupMenuItem(key: Key('notebook-delete'), value: 'delete', child: Text('Delete…')),
      ],
    );
  }
}
