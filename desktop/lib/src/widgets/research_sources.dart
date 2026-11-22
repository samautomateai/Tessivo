import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../core/core_client.dart';
import '../research_state.dart';
import '../theme.dart';
import 'text_prompt_dialog.dart';

/// The sources column: each source with its summary state and context level.
class ResearchSources extends StatelessWidget {
  const ResearchSources({super.key, required this.state});

  final ResearchState state;

  Future<void> _add(BuildContext context) async {
    final path = await askText(
      context,
      title: 'Add a source',
      label: 'Absolute path to a file or folder',
      maxLength: 4096,
    );
    if (path == null) return;
    try {
      checkContextPath(path);
    } on FormatException catch (e) {
      if (context.mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(e.message)));
      return;
    }
    await state.addSource(path);
  }

  @override
  Widget build(BuildContext context) {
    final sources = state.notebook?.sources ?? const <ResearchSource>[];
    return Column(
      key: const Key('research-sources'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 16, 8, 8),
          child: Row(
            children: [
              const Expanded(
                child: Text('Sources', style: TextStyle(fontWeight: FontWeight.w600)),
              ),
              TextButton.icon(
                key: const Key('research-add-source'),
                onPressed: () => unawaited(_add(context)),
                icon: const Icon(Icons.add, size: 16),
                label: const Text('Add'),
              ),
            ],
          ),
        ),
        Expanded(
          child: sources.isEmpty
              ? const Padding(
                  padding: EdgeInsets.all(16),
                  child: Text(
                    'Add a file or folder to ask about it. Each one gets a short summary.',
                    style: TextStyle(color: Tokens.textMuted),
                  ),
                )
              : ListView(
                  padding: const EdgeInsets.symmetric(horizontal: 8),
                  children: [for (final s in sources) _SourceCard(state: state, source: s)],
                ),
        ),
      ],
    );
  }
}

class _SourceCard extends StatelessWidget {
  const _SourceCard({required this.state, required this.source});

  final ResearchState state;
  final ResearchSource source;

  String get _status => switch (source.summary) {
    'ready' => source.tokens > 0 ? 'Summary ready · ≈ ${source.tokens ~/ 1000}k tokens in full' : 'Summary ready',
    'updating' => 'Updating summary…',
    'failed' => 'Summary failed${source.error == null ? '' : ': ${source.error}'}',
    'unreadable' => 'Cannot read this source${source.error == null ? '' : ': ${source.error}'}',
    _ => 'Summarizing…',
  };

  IconData get _icon => switch (source.kind) {
    'folder' => Icons.folder_outlined,
    'mcp-resource' => Icons.extension_outlined,
    _ => Icons.description_outlined,
  };

  @override
  Widget build(BuildContext context) {
    final s = source;
    return Card(
      key: Key('source-${s.id}'),
      margin: const EdgeInsets.only(bottom: 8),
      child: Padding(
        padding: const EdgeInsets.fromLTRB(12, 8, 4, 10),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Icon(_icon, size: 18),
                const SizedBox(width: 8),
                Expanded(
                  child: InkWell(
                    onTap: () => unawaited(showSourceText(context, state, s.id, s.label)),
                    child: Text(s.label, overflow: TextOverflow.ellipsis),
                  ),
                ),
                PopupMenuButton<String>(
                  key: Key('source-menu-${s.id}'),
                  tooltip: 'More',
                  onSelected: (v) => unawaited(v == 'remove' ? state.removeSource(s.id) : state.refreshSummary(s.id)),
                  itemBuilder: (_) => const [
                    PopupMenuItem(value: 'refresh', child: Text('Summarize again')),
                    PopupMenuItem(value: 'remove', child: Text('Remove')),
                  ],
                ),
              ],
            ),
            Text(
              _status,
              style: TextStyle(
                color: s.summary == 'failed' || s.summary == 'unreadable' ? Colors.red : Tokens.textMuted,
                fontSize: 12,
              ),
            ),
            const SizedBox(height: 6),
            Wrap(
              spacing: 6,
              children: [
                for (final (level, label) in const [('full', 'Full'), ('summary', 'Summary'), ('off', 'Off')])
                  ChoiceChip(
                    key: Key('level-${s.id}-$level'),
                    label: Text(label),
                    visualDensity: VisualDensity.compact,
                    selected: s.level == level,
                    onSelected: level == 'summary' && !s.hasSummary
                        ? null
                        : (_) => unawaited(state.setLevel(s.id, level)),
                  ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

/// Shows a source's text, the cited file of a folder when [path] is set, or
/// its summary when [summary] is true.
Future<void> showSourceText(
  BuildContext context,
  ResearchState state,
  String sourceId,
  String title, {
  String? path,
  bool summary = false,
}) async {
  final read = await state.sourceText(sourceId);
  if (read == null || !context.mounted) return;
  final body = summary && read.summary.isNotEmpty ? read.summary : fileSection(read.text, path);
  final heading = [title, ?path, if (summary) 'summary'].join(' · ');
  await showDialog<void>(
    context: context,
    builder: (ctx) => AlertDialog(
      key: const Key('source-dialog'),
      title: Text(heading),
      content: SizedBox(width: 720, height: 480, child: SingleChildScrollView(child: SelectableText(body))),
      actions: [TextButton(onPressed: () => Navigator.of(ctx).pop(), child: const Text('Close'))],
    ),
  );
}
