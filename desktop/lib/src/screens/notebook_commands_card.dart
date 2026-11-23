import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../app_state.dart';
import '../core/core_client.dart';
import '../core/models.dart';
import '../theme.dart';
import '../widgets/text_prompt_dialog.dart';

/// The notebook's agent-mode command allow-list: commands that run without
/// asking. Any other command the agent wants to run asks first.
class NotebookCommandsCard extends StatelessWidget {
  const NotebookCommandsCard({super.key, required this.state, required this.notebook});

  final AppState state;
  final Notebook notebook;

  Future<void> _save(BuildContext context, List<CommandRule> next) async {
    final messenger = ScaffoldMessenger.of(context);
    try {
      await state.setNotebookCommands(next);
    } on CoreException catch (e) {
      messenger.showSnackBar(SnackBar(content: Text(e.message)));
    }
  }

  Future<void> _add(BuildContext context) async {
    final line = await askText(
      context,
      title: 'Allow a command',
      label: 'Program and arguments, e.g. go test *  (* is one free argument)',
      maxLength: 4096,
    );
    final rule = line == null ? null : CommandRule.parse(line);
    if (rule != null && context.mounted) await _save(context, [...notebook.commands, rule]);
  }

  @override
  Widget build(BuildContext context) => Card(
    key: const Key('commands-card'),
    child: Padding(
      padding: const EdgeInsets.all(12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(child: Text('Agent commands', style: Theme.of(context).textTheme.titleSmall)),
              IconButton(
                key: const Key('command-add'),
                tooltip: 'Allow a command',
                onPressed: () => unawaited(_add(context)),
                icon: const Icon(Icons.add, size: 20),
              ),
            ],
          ),
          if (notebook.commands.isEmpty)
            const Text(
              'None. In agent mode every command asks first; add the ones that may run without asking.',
              style: TextStyle(color: Tokens.textMuted),
            ),
          for (final (i, c) in notebook.commands.indexed)
            ListTile(
              key: Key('command-$i'),
              dense: true,
              contentPadding: EdgeInsets.zero,
              title: Text('$c', style: const TextStyle(fontFamily: 'monospace')),
              trailing: IconButton(
                key: Key('command-remove-$i'),
                tooltip: 'Remove',
                icon: const Icon(Icons.close, size: 18),
                onPressed: () => unawaited(_save(context, [...notebook.commands]..removeAt(i))),
              ),
            ),
        ],
      ),
    ),
  );
}
