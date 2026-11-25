import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../app_state.dart';
import '../core/models.dart';
import '../design/components/buttons.dart';
import '../design/components/menu.dart';
import '../design/components/modal.dart';
import '../design/icons.dart';
import 'export_notebook_dialog.dart';
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

  /// Tags as one comma-separated line; up to 10, letters, digits, spaces, '-' and '_'.
  Future<void> _tags(BuildContext context) async {
    final line = await askText(
      context,
      title: 'Tags for "${notebook.name}"',
      label: 'Tags, separated by commas',
      initial: notebook.tags.join(', '),
      maxLength: 400,
    );
    if (line == null) return;
    final tags = [
      for (final t in line.split(','))
        if (t.trim().isNotEmpty) t.trim(),
    ];
    await state.setTags(notebook.id, tags);
  }

  Future<void> _delete(BuildContext context) async {
    final ok = await showTvModal<bool>(
      context,
      title: 'Delete "${notebook.name}"?',
      body: const Text(
        'This deletes the notebook folder with its sources, prompts, runs and outputs. It cannot be undone.',
      ),
      actions: const [
        TvModalAction('Cancel', value: false, kind: TvButtonKind.ghost),
        TvModalAction('Delete', value: true, kind: TvButtonKind.danger, key: Key('confirm-delete')),
      ],
    );
    if (ok ?? false) await state.deleteNotebook(notebook.id);
  }

  @override
  Widget build(BuildContext context) => TvMenuButton(
    key: const Key('notebook-menu'),
    label: 'Notebook actions',
    icon: TvIcon.more,
    compact: true,
    items: [
      TvMenuItem('Rename…', key: const Key('notebook-rename'), onSelected: () => unawaited(_rename(context))),
      TvMenuItem(
        notebook.pinned ? 'Unpin' : 'Pin',
        key: const Key('notebook-pin'),
        onSelected: () => unawaited(state.setPinned(notebook.id, !notebook.pinned)),
      ),
      TvMenuItem('Edit tags…', key: const Key('notebook-tags'), onSelected: () => unawaited(_tags(context))),
      TvMenuItem(
        'Export…',
        key: const Key('notebook-export'),
        onSelected: () => unawaited(exportNotebookDialog(context, state, notebook)),
      ),
      if (notebook.archived)
        TvMenuItem(
          'Restore from archive',
          key: const Key('notebook-restore'),
          onSelected: () => unawaited(state.setArchived(notebook.id, false)),
        )
      else
        TvMenuItem(
          'Archive',
          key: const Key('notebook-archive'),
          onSelected: () => unawaited(state.setArchived(notebook.id, true)),
        ),
      TvMenuItem('Delete…', key: const Key('notebook-delete'), onSelected: () => unawaited(_delete(context))),
    ],
  );
}
