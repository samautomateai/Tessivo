import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../core/core_client.dart';
import '../design/colors.dart';
import '../design/components/buttons.dart';
import '../design/components/table.dart';
import '../design/icons.dart';
import '../design/sizes.dart';
import '../design/type.dart';

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
    final c = TvColors.of(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Expanded(
              child: Text(
                'Attach a server to a notebook or chat to let it use the tools; every tool asks before it runs '
                'unless a rule allows it.',
                style: TvType.body.copyWith(color: c.textSecondary),
              ),
            ),
            const SizedBox(width: TvSpace.s16),
            TvButton('Add server', key: const Key('mcp-add'), icon: TvIcon.add, dense: true, onPressed: _add),
          ],
        ),
        const SizedBox(height: TvSpace.s16),
        FutureBuilder<List<McpServer>>(
          future: _servers,
          builder: (context, snap) {
            if (snap.hasError) {
              return Text('Could not load MCP servers: ${snap.error}', style: TvType.body.copyWith(color: c.error));
            }
            final list = snap.data ?? const <McpServer>[];
            if (list.isEmpty) return Text('No MCP servers yet.', style: TvType.body.copyWith(color: c.textSecondary));
            return TvDataTable(
              columns: const ['Server', 'Transport', 'Command or URL', ''],
              flex: const [2, 2, 6, 2],
              rows: [for (final s in list) _serverRow(s)],
            );
          },
        ),
      ],
    );
  }

  List<Widget> _serverRow(McpServer s) => [
    Text(s.name),
    Text(s.transport),
    Tooltip(
      message: s.target,
      child: Text(s.target, style: TvType.code),
    ),
    Row(
      mainAxisAlignment: MainAxisAlignment.end,
      children: [
        Flexible(
          child: TvButton(
            'Test',
            key: Key('mcp-test-${s.name}'),
            kind: TvButtonKind.ghost,
            dense: true,
            onPressed: () => unawaited(_test(s.name)),
          ),
        ),
        TvIconButton(
          TvIcon.close,
          key: Key('mcp-remove-${s.name}'),
          tooltip: 'Remove ${s.name}',
          onPressed: () => unawaited(_run(() => widget.core.removeMcpServer(s.name))),
        ),
      ],
    ),
  ];
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
            Text(
              r'Never paste a token here. Store it in your keychain or environment and write ${secret:NAME}, '
              r'for example Authorization=Bearer ${secret:GITHUB_TOKEN}.',
              style: TvType.helper.copyWith(color: TvColors.of(context).textHelper),
            ),
            if (_error case final e?)
              Padding(
                padding: const EdgeInsets.only(top: 8),
                child: Text(e, style: TvType.body.copyWith(color: TvColors.of(context).error)),
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
      Text('$title (${items.length})', style: TvType.heading01.copyWith(color: TvColors.of(context).textPrimary)),
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
