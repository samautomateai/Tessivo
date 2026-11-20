import 'package:material_ui/material_ui.dart';

import '../app_state.dart';
import '../core/models.dart';
import '../theme.dart';
import '../widgets/work_type_badge.dart';

/// Notebook workspace: context, prompt, run bar and streaming output.
class WorkspaceScreen extends StatefulWidget {
  const WorkspaceScreen({super.key, required this.state});

  final AppState state;

  @override
  State<WorkspaceScreen> createState() => _WorkspaceScreenState();
}

class _WorkspaceScreenState extends State<WorkspaceScreen> {
  final _prompt = TextEditingController(text: 'Assess the risk of merging this change.');

  @override
  void dispose() {
    _prompt.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final nb = widget.state.selected;
    if (nb == null) {
      return const Center(child: Text('Create a notebook to get started'));
    }
    return Row(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Expanded(
          child: ListView(
            padding: const EdgeInsets.all(24),
            children: [
              _Header(notebook: nb),
              const SizedBox(height: 16),
              _ContextCard(notebook: nb, onToggle: widget.state.toggleContext),
              const SizedBox(height: 16),
              _promptCard(),
              const SizedBox(height: 16),
              _runBar(nb),
            ],
          ),
        ),
        SizedBox(width: 420, child: _OutputPanel(state: widget.state)),
      ],
    );
  }

  Widget _promptCard() => Card(
    child: Padding(
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('Prompt', style: Theme.of(context).textTheme.titleSmall),
          const SizedBox(height: 8),
          TextField(
            key: const Key('prompt'),
            controller: _prompt,
            minLines: 3,
            maxLines: 8,
            decoration: const InputDecoration(border: OutlineInputBorder()),
          ),
        ],
      ),
    ),
  );

  Widget _runBar(Notebook nb) => Card(
    child: Padding(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
      child: Wrap(
        spacing: 10,
        runSpacing: 8,
        crossAxisAlignment: WrapCrossAlignment.center,
        children: [
          const Chip(label: Text('Model: smart')),
          const Chip(label: Text('Thinking: medium')),
          Text('≈ ${nb.enabledTokens ~/ 1000}k tokens', style: const TextStyle(color: Tokens.textMuted)),
          FilledButton(
            key: const Key('run'),
            onPressed: widget.state.running ? null : () => widget.state.run(_prompt.text, const {}),
            style: FilledButton.styleFrom(backgroundColor: Tokens.accent),
            child: Text(widget.state.running ? 'Running…' : 'Run'),
          ),
        ],
      ),
    ),
  );
}

class _Header extends StatelessWidget {
  const _Header({required this.notebook});

  final Notebook notebook;

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Flexible(
          child: Text(
            notebook.name,
            style: Theme.of(context).textTheme.headlineSmall?.copyWith(fontWeight: FontWeight.w600),
          ),
        ),
        const SizedBox(width: 12),
        WorkTypeBadge(workType: notebook.workType),
      ],
    );
  }
}

class _ContextCard extends StatelessWidget {
  const _ContextCard({required this.notebook, required this.onToggle});

  final Notebook notebook;
  final Future<void> Function(String id, bool enabled) onToggle;

  @override
  Widget build(BuildContext context) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('Context', style: Theme.of(context).textTheme.titleSmall),
            const SizedBox(height: 8),
            if (notebook.context.isEmpty)
              const Text('No context yet. Add files, folders, code bases or MCP servers.'),
            for (final c in notebook.context)
              CheckboxListTile(
                key: Key('ctx-${c.id}'),
                dense: true,
                value: c.enabled,
                onChanged: (v) => onToggle(c.id, v ?? false),
                title: Text(c.label),
                subtitle: Text(c.kind),
                secondary: Text('${c.estimatedTokens ~/ 1000}k tok'),
                controlAffinity: ListTileControlAffinity.leading,
              ),
          ],
        ),
      ),
    );
  }
}

class _OutputPanel extends StatelessWidget {
  const _OutputPanel({required this.state});

  final AppState state;

  @override
  Widget build(BuildContext context) {
    final outcome = state.outcome;
    return DecoratedBox(
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.surface,
        border: Border(left: BorderSide(color: Theme.of(context).dividerColor)),
      ),
      child: ListView(
        padding: const EdgeInsets.all(20),
        children: [
          Row(
            children: [
              Text('Output', style: Theme.of(context).textTheme.titleSmall),
              const Spacer(),
              if (outcome != null)
                Text(
                  outcome.valid ? 'Validated ✓' : 'Needs review',
                  style: TextStyle(color: outcome.valid ? const Color(0xFF1F7A4D) : Colors.orange),
                ),
            ],
          ),
          const SizedBox(height: 12),
          if (state.running) Text(state.status, style: const TextStyle(color: Tokens.textMuted)),
          if (state.streamedText.isEmpty && !state.running)
            const Text('Run the notebook to see its output here.', style: TextStyle(color: Tokens.textMuted)),
          Text(state.streamedText, key: const Key('output')),
          if (outcome != null) ...[
            const SizedBox(height: 16),
            Text(
              '${outcome.modelUsed} · ${outcome.totalTokens ~/ 1000}k tokens'
              '${outcome.fallbackNote == null ? '' : '\n${outcome.fallbackNote}'}',
              style: const TextStyle(color: Tokens.textMuted, fontSize: 12),
            ),
          ],
        ],
      ),
    );
  }
}
