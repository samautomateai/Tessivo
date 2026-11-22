import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../core/core_client.dart';
import '../theme.dart';

/// Settings › MCP servers: list, add, test and remove servers notebooks can
/// attach. Values never show credentials: the core refuses them as text.
class McpServersSection extends StatefulWidget {
  const McpServersSection({super.key, required this.core});

  final CoreClient core;

  @override
  State<McpServersSection> createState() => _McpServersSectionState();
}

class _McpServersSectionState extends State<McpServersSection> {
  late Future<List<McpServer>> _servers = widget.core.listMcpServers();

  void _reload() {
    final next = widget.core.listMcpServers();
    setState(() {
      _servers = next;
    });
  }

  Future<void> _run(Future<void> Function() action) async {
    final messenger = ScaffoldMessenger.of(context);
    try {
      await action();
      _reload();
    } on CoreException catch (e) {
      messenger.showSnackBar(SnackBar(content: Text(e.message)));
    }
  }

  Future<void> _add() async {
    final server = await showDialog<McpServer>(context: context, builder: (_) => const _AddServerDialog());
    if (server != null) await _run(() => widget.core.addMcpServer(server));
  }

  Future<void> _test(String name) async {
    final messenger = ScaffoldMessenger.of(context);
    try {
      final res = await widget.core.testMcpServer(name);
      if (!mounted) return;
      await showDialog<void>(
        context: context,
        builder: (_) => _TestResultDialog(name: name, result: res),
      );
    } on CoreException catch (e) {
      messenger.showSnackBar(SnackBar(content: Text('$name: ${e.message}')));
    }
  }

  @override
  Widget build(BuildContext context) {
    final text = Theme.of(context).textTheme;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Row(
          children: [
            Text('MCP servers', style: text.titleMedium),
            const Spacer(),
            TextButton.icon(
              key: const Key('mcp-add'),
              onPressed: _add,
              icon: const Icon(Icons.add, size: 18),
              label: const Text('Add server'),
            ),
          ],
        ),
        const Text(
          'Tools and resources from MCP servers. Attach a server to a notebook to let its agent runs use the tools; '
          'every tool asks before it runs unless a rule allows it.',
          style: TextStyle(color: Tokens.textMuted),
        ),
        const SizedBox(height: 8),
        FutureBuilder<List<McpServer>>(
          future: _servers,
          builder: (context, snap) {
            if (snap.hasError) return Text('Could not load MCP servers: ${snap.error}');
            final list = snap.data ?? const <McpServer>[];
            if (list.isEmpty) {
              return const Text('No MCP servers yet.', style: TextStyle(color: Tokens.textMuted));
            }
            return Column(children: [for (final s in list) _serverTile(s)]);
          },
        ),
      ],
    );
  }

  Widget _serverTile(McpServer s) => Card(
    child: ListTile(
      title: Text(s.name),
      subtitle: Text('${s.transport} · ${s.target}', maxLines: 1, overflow: TextOverflow.ellipsis),
      trailing: Wrap(
        spacing: 4,
        children: [
          TextButton(
            key: Key('mcp-test-${s.name}'),
            onPressed: () => unawaited(_test(s.name)),
            child: const Text('Test'),
          ),
          IconButton(
            key: Key('mcp-remove-${s.name}'),
            tooltip: 'Remove',
            icon: const Icon(Icons.delete_outline),
            onPressed: () => unawaited(_run(() => widget.core.removeMcpServer(s.name))),
          ),
        ],
      ),
    ),
  );
}

class _AddServerDialog extends StatefulWidget {
  const _AddServerDialog();

  @override
  State<_AddServerDialog> createState() => _AddServerDialogState();
}

class _AddServerDialogState extends State<_AddServerDialog> {
  final _name = TextEditingController();
  final _command = TextEditingController();
  final _url = TextEditingController();
  final _env = TextEditingController();
  final _headers = TextEditingController();
  String? _error;

  @override
  void dispose() {
    for (final c in [_name, _command, _url, _env, _headers]) {
      c.dispose();
    }
    super.dispose();
  }

  void _save() {
    final name = _name.text.trim();
    final command = _command.text.trim().split(RegExp(r'\s+')).where((p) => p.isNotEmpty).toList();
    final url = _url.text.trim();
    try {
      if (name.isEmpty) throw const FormatException('Give a name (a-z, 0-9, _ and -).');
      if (command.isEmpty == url.isEmpty) throw const FormatException('Give either a command or a URL.');
      Navigator.of(context).pop(
        McpServer(
          name: name,
          command: command.isEmpty ? '' : command.first,
          args: command.skip(1).toList(),
          url: url,
          env: parseKeyValues(_env.text),
          headers: parseKeyValues(_headers.text),
        ),
      );
    } on FormatException catch (e) {
      setState(() => _error = e.message);
    }
  }

  Widget _field(String key, TextEditingController c, String label, {String? hint, int lines = 1}) => Padding(
    padding: const EdgeInsets.only(bottom: 10),
    child: TextField(
      key: Key(key),
      controller: c,
      minLines: lines,
      maxLines: lines,
      decoration: InputDecoration(labelText: label, hintText: hint, border: const OutlineInputBorder()),
    ),
  );

  @override
  Widget build(BuildContext context) => AlertDialog(
    title: const Text('Add MCP server'),
    content: SizedBox(
      width: 520,
      child: SingleChildScrollView(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            _field('mcp-name', _name, 'Name', hint: 'github'),
            _field('mcp-command', _command, 'Command (stdio)', hint: 'npx -y @modelcontextprotocol/server-github'),
            _field('mcp-url', _url, 'or URL (Streamable HTTP)', hint: 'https://mcp.example.com/mcp'),
            _field('mcp-env', _env, 'Environment (KEY=VALUE per line)', lines: 2),
            _field('mcp-headers', _headers, 'Headers (KEY=VALUE per line)', lines: 2),
            const Text(
              r'Never paste a token here. Store it in your keychain or environment and write ${secret:NAME}, '
              r'for example Authorization=Bearer ${secret:GITHUB_TOKEN}.',
              style: TextStyle(color: Tokens.textMuted, fontSize: 12),
            ),
            if (_error case final e?)
              Padding(
                padding: const EdgeInsets.only(top: 8),
                child: Text(e, style: TextStyle(color: Theme.of(context).colorScheme.error)),
              ),
          ],
        ),
      ),
    ),
    actions: [
      TextButton(onPressed: () => Navigator.of(context).pop(), child: const Text('Cancel')),
      FilledButton(key: const Key('mcp-save'), onPressed: _save, child: const Text('Add')),
    ],
  );
}

class _TestResultDialog extends StatelessWidget {
  const _TestResultDialog({required this.name, required this.result});

  final String name;
  final McpTestResult result;

  Widget _section(BuildContext context, String title, List<McpEntry> items) => Column(
    crossAxisAlignment: CrossAxisAlignment.start,
    children: [
      Text('$title (${items.length})', style: Theme.of(context).textTheme.titleSmall),
      for (final e in items)
        Padding(
          padding: const EdgeInsets.only(left: 8, top: 2),
          child: Text(
            [e.uri.isEmpty ? e.name : e.uri, if (e.description.isNotEmpty) '— ${e.description}'].join(' '),
            maxLines: 2,
            overflow: TextOverflow.ellipsis,
          ),
        ),
      const SizedBox(height: 10),
    ],
  );

  @override
  Widget build(BuildContext context) => AlertDialog(
    title: Text('$name works'),
    content: SizedBox(
      width: 520,
      child: SingleChildScrollView(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            _section(context, 'Tools', result.tools),
            _section(context, 'Resources', result.resources),
            _section(context, 'Prompts', result.prompts),
          ],
        ),
      ),
    ),
    actions: [TextButton(onPressed: () => Navigator.of(context).pop(), child: const Text('Close'))],
  );
}
