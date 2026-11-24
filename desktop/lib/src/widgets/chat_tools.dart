import 'package:material_ui/material_ui.dart';

import '../app_state.dart';
import '../design/colors.dart';
import '../design/components/buttons.dart';
import '../design/components/fields.dart';
import '../design/type.dart';

/// Chooses the MCP servers whose tools the open chat may use.
Future<void> pickChatTools(BuildContext context, AppState state) async {
  final chat = state.chat;
  final chosen = await showDialog<List<String>>(
    context: context,
    builder: (_) =>
        _ServersDialog(all: [for (final s in state.mcpServers) s.name], selected: chat.open?.mcpServers ?? const []),
  );
  if (chosen != null) await chat.setMcpServers(chosen);
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
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    return AlertDialog(
      title: Text('Tools for this chat', style: TvType.heading02.copyWith(color: c.textPrimary)),
      content: SizedBox(
        width: 460,
        child: widget.all.isEmpty
            ? Text(
                'No MCP servers are configured. Add one in Settings.',
                style: TvType.body.copyWith(color: c.textSecondary),
              )
            : Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Text(
                    "The model may call these servers' tools. Tools that are not allowed by a rule ask you first.",
                    style: TvType.body.copyWith(color: c.textSecondary),
                  ),
                  const SizedBox(height: 8),
                  for (final name in widget.all)
                    TvCheckbox(
                      key: Key('chat-tool-$name'),
                      label: name,
                      value: _on.contains(name),
                      onChanged: (v) => setState(() => v ? _on.add(name) : _on.remove(name)),
                    ),
                ],
              ),
      ),
      actions: [
        TvButton('Cancel', kind: TvButtonKind.ghost, onPressed: () => Navigator.of(context).pop()),
        TvButton(
          'Save',
          key: const Key('chat-tools-save'),
          kind: TvButtonKind.primary,
          onPressed: () => Navigator.of(context).pop([
            for (final n in widget.all)
              if (_on.contains(n)) n,
          ]),
        ),
      ],
    );
  }
}
