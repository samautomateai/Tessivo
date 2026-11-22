import 'package:material_ui/material_ui.dart';

import '../app_state.dart';
import '../core/models.dart';
import '../theme.dart';
import '../widgets/approvals_sheet.dart';
import '../widgets/notebook_menu.dart';
import '../widgets/output_thread.dart';
import '../widgets/prompt_menu.dart';
import 'notebook_outputs_pane.dart';
import 'notebook_sources_pane.dart';

/// Notebook page: sources on the left; the run bar, the open output and its
/// thread in the middle; the outputs on the right. Below [wideWidth] the side
/// panes collapse behind toggles, one at a time.
class WorkspaceScreen extends StatefulWidget {
  const WorkspaceScreen({super.key, required this.state});

  final AppState state;

  /// Narrowest content width that shows all three panes.
  static const wideWidth = 1100.0;

  @override
  State<WorkspaceScreen> createState() => _WorkspaceScreenState();
}

enum _SidePane { none, sources, outputs }

class _WorkspaceScreenState extends State<WorkspaceScreen> {
  final _prompt = TextEditingController(text: 'Assess the risk of merging this change.');
  _SidePane _shown = _SidePane.none;

  /// The saved prompt in the box, until its text is edited.
  String? _promptName;

  @override
  void initState() {
    super.initState();
    _prompt.addListener(_detach);
  }

  void _detach() {
    final p = widget.state.prompts.where((p) => p.name == _promptName).firstOrNull;
    if (_promptName != null && p?.text != _prompt.text) setState(() => _promptName = null);
  }

  void _use(SavedPrompt p) {
    _prompt.text = p.text;
    setState(() => _promptName = p.name);
  }

  @override
  void dispose() {
    _prompt.dispose();
    super.dispose();
  }

  void _toggle(_SidePane p) => setState(() => _shown = _shown == p ? _SidePane.none : p);

  @override
  Widget build(BuildContext context) {
    final state = widget.state;
    final nb = state.selected;
    if (nb == null) {
      return const Center(child: Text('Create a notebook to get started'));
    }
    return LayoutBuilder(
      builder: (context, box) {
        final wide = box.maxWidth >= WorkspaceScreen.wideWidth;
        return Row(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            if (wide || _shown == _SidePane.sources) NotebookSourcesPane(state: state, notebook: nb),
            Expanded(child: _middle(nb, wide)),
            if (wide || _shown == _SidePane.outputs) NotebookOutputsPane(state: state),
          ],
        );
      },
    );
  }

  Widget _middle(Notebook nb, bool wide) {
    final state = widget.state;
    final thread = state.thread;
    final open = state.outputs.where((o) => o.id == state.selectedOutputId).firstOrNull;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 12, 16, 0),
          child: Row(
            children: [
              if (!wide)
                IconButton(
                  key: const Key('toggle-sources'),
                  tooltip: 'Sources',
                  onPressed: () => _toggle(_SidePane.sources),
                  icon: const Icon(Icons.folder_open),
                ),
              Expanded(child: _runBar(nb)),
              if (!wide)
                IconButton(
                  key: const Key('toggle-outputs'),
                  tooltip: 'Outputs',
                  onPressed: () => _toggle(_SidePane.outputs),
                  icon: const Icon(Icons.history),
                ),
              NotebookMenu(state: state, notebook: nb),
            ],
          ),
        ),
        _RunPanel(state: state),
        const Divider(height: 1),
        Expanded(
          child: thread == null
              ? const Center(
                  child: Text(
                    'Run the notebook to get an output, then ask about it here.',
                    style: TextStyle(color: Tokens.textMuted),
                  ),
                )
              : OutputThread(key: ValueKey(thread), state: state, thread: thread, version: open?.version ?? 1),
        ),
      ],
    );
  }

  Widget _runBar(Notebook nb) => Card(
    child: Padding(
      padding: const EdgeInsets.all(12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          TextField(
            key: const Key('prompt'),
            controller: _prompt,
            minLines: 1,
            maxLines: 4,
            decoration: const InputDecoration(labelText: 'Prompt', border: OutlineInputBorder()),
          ),
          const SizedBox(height: 8),
          Wrap(
            spacing: 10,
            runSpacing: 8,
            crossAxisAlignment: WrapCrossAlignment.center,
            children: [
              PromptMenu(state: widget.state, text: () => _prompt.text, onUse: _use),
              if (_promptName != null) Chip(label: Text('Prompt: $_promptName')),
              Text('≈ ${nb.enabledTokens ~/ 1000}k tokens', style: const TextStyle(color: Tokens.textMuted)),
              FilledButton(
                key: const Key('run'),
                onPressed: widget.state.running
                    ? null
                    : () => widget.state.run(_prompt.text, const {}, promptName: _promptName),
                style: FilledButton.styleFrom(backgroundColor: Tokens.accent),
                child: Text(widget.state.running ? 'Running…' : 'Run'),
              ),
            ],
          ),
        ],
      ),
    ),
  );
}

/// The run in progress (or just finished): status, tool calls, approvals and
/// the streamed text. Hidden when there is nothing to show.
class _RunPanel extends StatelessWidget {
  const _RunPanel({required this.state});

  final AppState state;

  @override
  Widget build(BuildContext context) {
    final outcome = state.outcome;
    final active = state.running || outcome != null || state.streamedText.isNotEmpty || state.paused.isNotEmpty;
    if (!active) return const SizedBox.shrink();
    return ConstrainedBox(
      constraints: const BoxConstraints(maxHeight: 260),
      child: ListView(
        shrinkWrap: true,
        padding: const EdgeInsets.fromLTRB(20, 8, 20, 12),
        children: [
          Row(
            children: [
              Text('Latest run', style: Theme.of(context).textTheme.titleSmall),
              const Spacer(),
              if (outcome != null)
                Text(
                  outcome.valid ? 'Validated ✓' : 'Needs review',
                  style: TextStyle(color: outcome.valid ? const Color(0xFF1F7A4D) : Colors.orange),
                ),
            ],
          ),
          if (state.running) Text(state.status, style: const TextStyle(color: Tokens.textMuted)),
          for (final line in state.toolLog)
            Text(
              line,
              style: const TextStyle(color: Tokens.textMuted, fontFamily: 'monospace', fontSize: 12),
            ),
          if (state.paused.isNotEmpty) ...[
            const SizedBox(height: 8),
            Text('Waiting for approval', style: Theme.of(context).textTheme.titleSmall),
            for (final a in state.paused) ApprovalTile(state: state, approval: a),
          ],
          Text(state.streamedText, key: const Key('output')),
          if (outcome != null)
            Text(
              '${outcome.modelUsed} · ${outcome.totalTokens ~/ 1000}k tokens'
              '${outcome.fallbackNote == null ? '' : '\n${outcome.fallbackNote}'}',
              style: const TextStyle(color: Tokens.textMuted, fontSize: 12),
            ),
        ],
      ),
    );
  }
}
