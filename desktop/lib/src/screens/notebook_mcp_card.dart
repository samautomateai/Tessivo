import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../app_state.dart';
import '../core/core_client.dart';
import '../core/models.dart';
import '../theme.dart';

/// The notebook's MCP servers: which ones its agent runs may use, and a way to
/// add one of their resources as a source.
class NotebookMcpCard extends StatelessWidget {
  const NotebookMcpCard({super.key, required this.state, required this.notebook});

  final AppState state;
  final Notebook notebook;

  Future<void> _guard(BuildContext context, Future<void> Function() action) async {
    final messenger = ScaffoldMessenger.of(context);
    try {
      await action();
    } on Object catch (e) {
      messenger.showSnackBar(SnackBar(content: Text(e is CoreException ? e.message : '$e')));
    }
  }

  Future<void> _addResource(BuildContext context) async {
    final picked = await showDialog<(String, String)>(
      context: context,
      builder: (_) => _ResourceDialog(servers: [for (final s in state.mcpServers) s.name]),
    );
    if (picked != null && context.mounted) {
      await _guard(context, () => state.addMcpContext(picked.$1, picked.$2));
    }
  }

  @override
  Widget build(BuildContext context) {
    final servers = state.mcpServers;
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Expanded(
                  child: Text(
                    'MCP servers',
                    style: Theme.of(context).textTheme.titleSmall,
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
                IconButton(
                  key: const Key('context-add-mcp'),
                  tooltip: 'Add an MCP resource as a source',
                  onPressed: servers.isEmpty ? null : () => unawaited(_addResource(context)),
                  icon: const Icon(Icons.add_link, size: 20),
                ),
              ],
            ),
            if (servers.isEmpty)
              const Text('Add MCP servers in Settings to use them here.', style: TextStyle(color: Tokens.textMuted)),
            for (final s in servers)
              SwitchListTile(
                key: Key('nb-mcp-${s.name}'),
                dense: true,
                contentPadding: EdgeInsets.zero,
                title: Text(s.name),
                subtitle: const Text('tools for agent runs'),
                value: notebook.mcpServers.contains(s.name),
                onChanged: (on) => unawaited(
                  _guard(context, () {
                    final next = [...notebook.mcpServers.where((n) => n != s.name), if (on) s.name];
                    return state.setNotebookMcp(next);
                  }),
                ),
              ),
          ],
        ),
      ),
    );
  }
}

class _ResourceDialog extends StatefulWidget {
  const _ResourceDialog({required this.servers});

  final List<String> servers;

  @override
  State<_ResourceDialog> createState() => _ResourceDialogState();
}

class _ResourceDialogState extends State<_ResourceDialog> {
  late String _server = widget.servers.first;
  final _uri = TextEditingController();

  @override
  void dispose() {
    _uri.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => AlertDialog(
    title: const Text('Add an MCP resource'),
    content: SizedBox(
      width: 460,
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          DropdownButtonFormField<String>(
            initialValue: _server,
            decoration: const InputDecoration(labelText: 'Server', border: OutlineInputBorder()),
            items: [for (final s in widget.servers) DropdownMenuItem(value: s, child: Text(s))],
            onChanged: (v) => setState(() => _server = v ?? _server),
          ),
          const SizedBox(height: 10),
          TextField(
            key: const Key('mcp-resource-uri'),
            controller: _uri,
            decoration: const InputDecoration(
              labelText: 'Resource URI',
              hintText: 'file:///notes.md',
              border: OutlineInputBorder(),
            ),
          ),
          const SizedBox(height: 6),
          const Text(
            "It is read on every run, so the notebook always sees the server's current version.",
            style: TextStyle(color: Tokens.textMuted, fontSize: 12),
          ),
        ],
      ),
    ),
    actions: [
      TextButton(onPressed: () => Navigator.of(context).pop(), child: const Text('Cancel')),
      FilledButton(
        key: const Key('mcp-resource-add'),
        onPressed: () {
          final uri = _uri.text.trim();
          if (uri.isNotEmpty) Navigator.of(context).pop((_server, uri));
        },
        child: const Text('Add'),
      ),
    ],
  );
}
