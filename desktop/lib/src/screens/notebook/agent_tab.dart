import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../../app_state.dart';
import '../../core/models.dart';
import '../../design/colors.dart';
import '../../design/components/buttons.dart';
import '../../design/components/fields.dart';
import '../../design/icons.dart';
import '../../design/sizes.dart';
import '../../design/type.dart';
import '../../widgets/text_prompt_dialog.dart';
import 'tab_section.dart';

/// The Agent tab: the MCP servers agent runs may use, and the commands that
/// run without asking. Any other command asks first.
class AgentTab extends StatelessWidget {
  const AgentTab({super.key, required this.state, required this.notebook});

  final AppState state;
  final Notebook notebook;

  Future<void> _addCommand(BuildContext context) async {
    final line = await askText(
      context,
      title: 'Allow a command',
      label: 'Program and arguments, e.g. go test *  (* is one free argument)',
      maxLength: 4096,
    );
    final rule = line == null ? null : CommandRule.parse(line);
    if (rule == null || !context.mounted) return;
    await guarded(context, () => state.setNotebookCommands([...notebook.commands, rule]));
  }

  Future<void> _setServer(BuildContext context, String name, bool on) =>
      guarded(context, () => state.setNotebookMcp([...notebook.mcpServers.where((n) => n != name), if (on) name]));

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    final servers = state.mcpServers;
    final commands = notebook.commands;
    return TabBody(
      children: [
        TabSection(
          title: 'MCP servers',
          helper: 'Tools agent runs of this notebook may call.',
          child: servers.isEmpty
              ? const EmptyNote('Add MCP servers in Settings to use them here.')
              : Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    for (final s in servers)
                      TvToggle(
                        key: Key('nb-mcp-${s.name}'),
                        label: s.name,
                        value: notebook.mcpServers.contains(s.name),
                        onChanged: (on) => unawaited(_setServer(context, s.name, on)),
                      ),
                  ],
                ),
        ),
        const SizedBox(height: TvSpace.s32),
        TabSection(
          key: const Key('commands-card'),
          title: 'Commands that run without asking',
          helper: 'In agent mode every other command asks first.',
          actions: [
            TvButton(
              'Allow a command',
              key: const Key('command-add'),
              icon: TvIcon.add,
              dense: true,
              onPressed: () => unawaited(_addCommand(context)),
            ),
          ],
          child: commands.isEmpty
              ? const EmptyNote('None yet. Add the commands the agent may run without asking, like go test *.')
              : Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    for (final (i, rule) in commands.indexed)
                      Container(
                        key: Key('command-$i'),
                        height: TvSize.tableRow,
                        padding: const EdgeInsets.only(left: TvSpace.s12, right: TvSpace.s4),
                        decoration: BoxDecoration(
                          color: c.layer01,
                          border: Border(bottom: BorderSide(color: c.borderSubtle)),
                        ),
                        child: Row(
                          children: [
                            TvIconView(TvIcon.terminal, size: 14, color: c.textSecondary),
                            const SizedBox(width: TvSpace.s8),
                            Expanded(
                              child: Text(
                                '$rule',
                                maxLines: 1,
                                overflow: TextOverflow.ellipsis,
                                style: TvType.code.copyWith(color: c.textPrimary),
                              ),
                            ),
                            TvIconButton(
                              TvIcon.close,
                              key: Key('command-remove-$i'),
                              tooltip: 'Remove',
                              onPressed: () => unawaited(
                                guarded(context, () => state.setNotebookCommands([...commands]..removeAt(i))),
                              ),
                            ),
                          ],
                        ),
                      ),
                  ],
                ),
        ),
      ],
    );
  }
}
