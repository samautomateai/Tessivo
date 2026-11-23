import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../app_state.dart';
import '../chat_state.dart';
import '../theme.dart';

/// Chooses the MCP servers whose tools the open chat may use.
class ChatToolsButton extends StatelessWidget {
  const ChatToolsButton({super.key, required this.state});

  final AppState state;

  Future<void> _pick(BuildContext context) async {
    final chat = state.chat;
    final chosen = await showDialog<List<String>>(
      context: context,
      builder: (_) =>
          _ServersDialog(all: [for (final s in state.mcpServers) s.name], selected: chat.open?.mcpServers ?? const []),
    );
    if (chosen != null) await chat.setMcpServers(chosen);
  }

  @override
  Widget build(BuildContext context) {
    final n = state.chat.open?.mcpServers.length ?? 0;
    return IconButton(
      key: const Key('chat-tools'),
      tooltip: n == 0 ? 'Tools: none' : 'Tools: $n MCP server(s)',
      onPressed: state.chat.sending ? null : () => unawaited(_pick(context)),
      icon: Badge(isLabelVisible: n > 0, label: Text('$n'), child: const Icon(Icons.extension_outlined)),
    );
  }
}

class _ServersDialog extends StatefulWidget {
  const _ServersDialog({required this.all, required this.selected});

  final List<String> all;
  final List<String> selected;

  @override
  State<_ServersDialog> createState() => _ServersDialogState();
}

class _ServersDialogState extends State<_ServersDialog> {
  late final Set<String> _on = {...widget.selected};

  @override
  Widget build(BuildContext context) => AlertDialog(
    title: const Text('Tools for this chat'),
    content: SizedBox(
      width: 460,
      child: widget.all.isEmpty
          ? const Text('No MCP servers are configured. Add one in Settings.')
          : Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                const Text(
                  'The model may call these servers\' tools. Tools that are not allowed by a rule ask you first.',
                  style: TextStyle(color: Tokens.textMuted),
                ),
                for (final name in widget.all)
                  CheckboxListTile(
                    key: Key('chat-tool-$name'),
                    value: _on.contains(name),
                    title: Text(name),
                    onChanged: (v) => setState(() => v == true ? _on.add(name) : _on.remove(name)),
                  ),
              ],
            ),
    ),
    actions: [
      TextButton(onPressed: () => Navigator.of(context).pop(), child: const Text('Cancel')),
      FilledButton(
        key: const Key('chat-tools-save'),
        onPressed: () => Navigator.of(context).pop([
          for (final n in widget.all)
            if (_on.contains(n)) n,
        ]),
        child: const Text('Save'),
      ),
    ],
  );
}

/// The reply's tool activity and, when one waits, the approval card.
class ChatToolPanel extends StatelessWidget {
  const ChatToolPanel({super.key, required this.chat});

  final ChatState chat;

  @override
  Widget build(BuildContext context) {
    final a = chat.approval;
    if (chat.toolLog.isEmpty && a == null) return const SizedBox.shrink();
    return Padding(
      padding: const EdgeInsets.fromLTRB(20, 8, 20, 4),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          for (final (i, t) in chat.toolLog.indexed)
            Text(
              key: Key('tool-line-$i'),
              '${t.kind == 'result' ? '←' : '→'} ${t.tool} ${t.kind == 'result' ? t.summary : t.kind}',
              style: const TextStyle(color: Tokens.textMuted, fontSize: 12),
            ),
          if (a != null)
            Card(
              key: const Key('approval-card'),
              child: Padding(
                padding: const EdgeInsets.all(12),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text('Allow ${a.tool}?', style: Theme.of(context).textTheme.titleSmall),
                    if (a.args.isNotEmpty)
                      Text(a.args, maxLines: 4, overflow: TextOverflow.ellipsis, style: const TextStyle(fontSize: 12)),
                    const SizedBox(height: 8),
                    Wrap(
                      spacing: 8,
                      children: [
                        FilledButton(
                          key: const Key('tool-allow'),
                          onPressed: () => unawaited(chat.answerTool(allow: true)),
                          child: const Text('Allow once'),
                        ),
                        OutlinedButton(
                          key: const Key('tool-always'),
                          onPressed: () => unawaited(chat.answerTool(allow: true, remember: true)),
                          child: const Text('Always in this chat'),
                        ),
                        TextButton(
                          key: const Key('tool-deny'),
                          onPressed: () => unawaited(chat.answerTool(allow: false)),
                          child: const Text('Deny'),
                        ),
                      ],
                    ),
                  ],
                ),
              ),
            ),
        ],
      ),
    );
  }
}
