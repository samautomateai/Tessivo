import 'package:material_ui/material_ui.dart';

import '../app_state.dart';
import '../core/core_client.dart';
import '../core/models.dart';
import '../widgets/git_diff_dialog.dart';
import '../widgets/memory_card.dart';
import '../widgets/paste_text_dialog.dart';
import '../widgets/text_prompt_dialog.dart';
import '../widgets/work_type_badge.dart';
import 'notebook_commands_card.dart';
import 'notebook_mcp_card.dart';

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
          ContextCard(
            notebook: notebook,
            onToggle: state.toggleContext,
            onAdd: state.addContext,
            onAddUrl: state.addUrlContext,
            onAddText: state.addTextContext,
            onAddGit: state.addGitDiffContext,
          ),
          const SizedBox(height: 16),
          MemoryCard(notebook: notebook, onChanged: state.setMemory),
          const SizedBox(height: 16),
          NotebookMcpCard(state: state, notebook: notebook),
          const SizedBox(height: 16),
          NotebookCommandsCard(state: state, notebook: notebook),
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
  const ContextCard({
    super.key,
    required this.notebook,
    required this.onToggle,
    required this.onAdd,
    this.onAddUrl,
    this.onAddText,
    this.onAddGit,
  });

  final Notebook notebook;
  final Future<void> Function(String id, bool enabled) onToggle;
  final Future<void> Function(String path) onAdd;
  final Future<void> Function(String url)? onAddUrl;
  final Future<void> Function(String label, String text)? onAddText;
  final Future<void> Function(String repo, String base)? onAddGit;

  Future<void> _addMore(BuildContext context, String what) async {
    final messenger = ScaffoldMessenger.of(context);
    try {
      if (what == 'url') {
        final url = await askText(context, title: 'Add a web page', label: 'URL (http or https)', maxLength: 2048);
        if (url != null && url.trim().isNotEmpty) await onAddUrl?.call(url.trim());
      } else if (what == 'git') {
        final got = await showDialog<(String, String)>(context: context, builder: (_) => const GitDiffDialog());
        if (got != null && got.$1.isNotEmpty) await onAddGit?.call(got.$1, got.$2.isEmpty ? 'main' : got.$2);
      } else {
        final got = await showDialog<(String, String)>(context: context, builder: (_) => const PasteTextDialog());
        if (got != null && got.$2.trim().isNotEmpty) await onAddText?.call(got.$1, got.$2);
      }
    } on CoreException catch (e) {
      messenger.showSnackBar(SnackBar(content: Text(e.message)));
    }
  }

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
                Expanded(child: Text('Sources', style: Theme.of(context).textTheme.titleSmall)),
                TextButton.icon(
                  key: const Key('context-add'),
                  onPressed: () => _add(context),
                  icon: const Icon(Icons.add, size: 18),
                  label: const Text('Add'),
                ),
                PopupMenuButton<String>(
                  key: const Key('context-add-more'),
                  tooltip: 'More sources',
                  padding: EdgeInsets.zero,
                  constraints: const BoxConstraints(minWidth: 28, minHeight: 28),
                  icon: const Icon(Icons.expand_more, size: 18),
                  onSelected: (v) => _addMore(context, v),
                  itemBuilder: (_) => const [
                    PopupMenuItem(key: Key('context-add-url'), value: 'url', child: Text('Web page…')),
                    PopupMenuItem(key: Key('context-add-text'), value: 'text', child: Text('Paste text…')),
                    PopupMenuItem(key: Key('context-add-git'), value: 'git', child: Text('Git changes…')),
                  ],
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
                subtitle: Text(
                  c.readAtRunTime ? '${c.kind} · read at run time' : '${c.kind} · ${c.estimatedTokens ~/ 1000}k tok',
                ),
                controlAffinity: ListTileControlAffinity.leading,
              ),
          ],
        ),
      ),
    );
  }
}
