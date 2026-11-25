import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../../app_state.dart';
import '../../core/models.dart';
import '../../design/colors.dart';
import '../../design/components/buttons.dart';
import '../../design/components/fields.dart';
import '../../design/components/menu.dart';
import '../../design/components/table.dart';
import '../../design/icons.dart';
import '../../design/sizes.dart';
import '../../design/type.dart';
import '../../widgets/git_diff_dialog.dart';
import '../../widgets/paste_text_dialog.dart';
import '../../widgets/text_prompt_dialog.dart';
import 'tab_section.dart';

/// What a source costs a run: its token estimate, or "read at run time".
String sourceSize(ContextItem c) => c.readAtRunTime ? 'read at run time' : '≈ ${c.estimatedTokens ~/ 1000}k tokens';

/// The watch modes (SRC-8) and their labels.
const watchModes = [('off', 'Do nothing'), ('notify', 'Tell me'), ('run', 'Tell me and run the notebook')];

/// The Sources tab: every source with its include box, kind and size, and
/// Add (a path) with more kinds behind the menu next to it.
class SourcesTab extends StatelessWidget {
  const SourcesTab({super.key, required this.state, required this.notebook});

  final AppState state;
  final Notebook notebook;

  Future<void> _addPath(BuildContext context) async {
    final path = await askText(
      context,
      title: 'Add a file or folder',
      label: 'Absolute path to a file or folder',
      maxLength: 4096,
    );
    if (path != null && path.trim().isNotEmpty && context.mounted) {
      await guarded(context, () => state.addContext(path.trim()));
    }
  }

  Future<void> _addMore(BuildContext context, String what) async {
    switch (what) {
      case 'url':
        final url = await askText(context, title: 'Add a web page', label: 'URL (http or https)', maxLength: 2048);
        if (url != null && url.trim().isNotEmpty && context.mounted) {
          await guarded(context, () => state.addUrlContext(url.trim()));
        }
      case 'git':
        final got = await showDialog<(String, String)>(context: context, builder: (_) => const GitDiffDialog());
        if (got != null && got.$1.isNotEmpty && context.mounted) {
          await guarded(context, () => state.addGitDiffContext(got.$1, got.$2.isEmpty ? 'main' : got.$2));
        }
      case 'mcp':
        final got = await showDialog<(String, String)>(
          context: context,
          builder: (_) => McpResourceDialog(servers: [for (final s in state.mcpServers) s.name]),
        );
        if (got != null && context.mounted) await guarded(context, () => state.addMcpContext(got.$1, got.$2));
      default:
        final got = await showDialog<(String, String)>(context: context, builder: (_) => const PasteTextDialog());
        if (got != null && got.$2.trim().isNotEmpty && context.mounted) {
          await guarded(context, () => state.addTextContext(got.$1, got.$2));
        }
    }
  }

  @override
  Widget build(BuildContext context) {
    final sources = notebook.context;
    void more(String what) => unawaited(_addMore(context, what));
    return TabBody(
      children: [
        TabSection(
          title: 'Sources',
          helper: 'Included sources go with every run of this notebook.',
          actions: [
            TvButton(
              'Add',
              key: const Key('context-add'),
              icon: TvIcon.add,
              dense: true,
              onPressed: () => unawaited(_addPath(context)),
            ),
            TvMenuButton(
              key: const Key('context-add-more'),
              label: 'More sources',
              icon: TvIcon.expand,
              items: [
                TvMenuItem('Web page…', key: const Key('context-add-url'), onSelected: () => more('url')),
                TvMenuItem('Paste text…', key: const Key('context-add-text'), onSelected: () => more('text')),
                TvMenuItem('Git changes…', key: const Key('context-add-git'), onSelected: () => more('git')),
                TvMenuItem(
                  'MCP resource…',
                  key: const Key('context-add-mcp'),
                  enabled: state.mcpServers.isNotEmpty,
                  onSelected: () => more('mcp'),
                ),
              ],
            ),
          ],
          child: sources.isEmpty
              ? const EmptyNote('No sources yet. Add files, folders, web pages or git changes')
              : TvDataTable(
                  columns: const ['Source', 'Kind', 'Size'],
                  flex: const [6, 2, 2],
                  rows: [
                    for (final c in sources)
                      [
                        TvCheckbox(
                          key: Key('ctx-${c.id}'),
                          label: c.label,
                          value: c.enabled,
                          onChanged: (v) => unawaited(guarded(context, () => state.toggleContext(c.id, v))),
                        ),
                        Text(c.kind),
                        Text(sourceSize(c)),
                      ],
                  ],
                ),
        ),
        const SizedBox(height: TvSpace.s24),
        TabSection(
          title: 'Watch',
          helper: 'Checks folders and files used in place every minute while Tessivo runs.',
          child: Align(
            alignment: Alignment.centerLeft,
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 360),
              child: TvDropdown<String>(
                key: const Key('watch-mode'),
                label: 'When the sources change',
                items: watchModes,
                value: notebook.watch.isEmpty ? 'off' : notebook.watch,
                onChanged: (v) => unawaited(state.setWatch(notebook.id, v)),
              ),
            ),
          ),
        ),
      ],
    );
  }
}

/// Picks an MCP server and a resource URI to add as a source.
class McpResourceDialog extends StatefulWidget {
  const McpResourceDialog({super.key, required this.servers});

  final List<String> servers;

  @override
  State<McpResourceDialog> createState() => _McpResourceDialogState();
}

class _McpResourceDialogState extends State<McpResourceDialog> {
  late String _server = widget.servers.first;
  final _uri = TextEditingController();

  @override
  void dispose() {
    _uri.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    return AlertDialog(
      title: Text('Add an MCP resource', style: TvType.heading02.copyWith(color: c.textPrimary)),
      content: SizedBox(
        width: 460,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            TvDropdown<String>(
              label: 'Server',
              items: [for (final s in widget.servers) (s, s)],
              value: _server,
              onChanged: (v) => setState(() => _server = v),
            ),
            const SizedBox(height: TvSpace.s12),
            TvTextField(
              label: 'Resource URI',
              fieldKey: const Key('mcp-resource-uri'),
              controller: _uri,
              hint: 'file:///notes.md',
              helper: "It is read on every run, so the notebook always sees the server's current version.",
            ),
          ],
        ),
      ),
      actions: [
        TvButton('Cancel', kind: TvButtonKind.ghost, onPressed: () => Navigator.of(context).pop()),
        TvButton(
          'Add',
          key: const Key('mcp-resource-add'),
          kind: TvButtonKind.primary,
          onPressed: () {
            final uri = _uri.text.trim();
            if (uri.isNotEmpty) Navigator.of(context).pop((_server, uri));
          },
        ),
      ],
    );
  }
}
