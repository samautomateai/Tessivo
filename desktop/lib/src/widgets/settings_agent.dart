import 'package:material_ui/material_ui.dart';

import '../core/models.dart';
import '../core/settings_draft.dart';
import '../theme.dart';

/// Agent-mode limits and the default rule for each tool, saved together.
/// Notebooks and single runs can set stricter or extra rules of their own.
class AgentEditor extends StatefulWidget {
  const AgentEditor({super.key, required this.settings, required this.onSave});

  final SettingsInfo settings;

  /// Saves the edited draft; returns an error message or null.
  final Future<String?> Function(SettingsDraft draft) onSave;

  @override
  State<AgentEditor> createState() => _AgentEditorState();
}

/// The limits shown, as config key, label and built-in default.
const _limits = [
  ('max_steps', 'Max steps', 40),
  ('max_tool_calls', 'Max tool calls', 200),
  ('max_tokens', 'Max tokens', 400000),
  ('max_minutes', 'Max minutes', 30),
  ('approval_expiry_days', 'Approvals expire after (days)', 7),
];

const _rules = ['allow', 'ask', 'block'];

class _AgentEditorState extends State<AgentEditor> {
  late final SettingsDraft _start = SettingsDraft.fromJson(widget.settings.raw);
  late final Map<String, TextEditingController> _nums = {
    for (final (key, _, _) in _limits) key: TextEditingController(text: '${_start.agentLimit(key) ?? ''}'),
  };
  late final List<(TextEditingController, String)> _tools = [
    for (final e in _start.toolRules.entries) (TextEditingController(text: e.key), e.value),
  ];
  String? _error;
  String? _status;

  @override
  void dispose() {
    for (final c in [..._nums.values, for (final (c, _) in _tools) c]) {
      c.dispose();
    }
    super.dispose();
  }

  Future<void> _save() async {
    final draft = SettingsDraft.fromJson(widget.settings.raw);
    for (final MapEntry(:key, value: c) in _nums.entries) {
      final t = c.text.trim();
      final n = t.isEmpty ? 0 : int.tryParse(t);
      if (n == null || n < 0) {
        setState(() => _error = 'Limits must be whole numbers');
        return;
      }
      draft.setAgentLimit(key, n);
    }
    final rules = <String, String>{};
    for (final (c, rule) in _tools) {
      final name = c.text.trim();
      if (name.isEmpty) continue;
      if (rules.containsKey(name)) {
        setState(() => _error = 'Tool "$name" has two rules');
        return;
      }
      rules[name] = rule;
    }
    draft.toolRules = rules;
    final err = await widget.onSave(draft);
    if (!mounted) return;
    setState(() {
      _error = null;
      _status = err ?? 'Saved';
    });
  }

  Widget _toolRow(int i) {
    final (c, rule) = _tools[i];
    return Row(
      children: [
        Expanded(
          child: TextField(
            key: Key('tool-name-$i'),
            controller: c,
            decoration: const InputDecoration(hintText: 'read_file, run_command or mcp__server__tool'),
          ),
        ),
        const SizedBox(width: 12),
        DropdownButton<String>(
          key: Key('tool-rule-$i'),
          value: rule,
          items: [for (final r in _rules) DropdownMenuItem(value: r, child: Text(r))],
          onChanged: (v) => setState(() => _tools[i] = (c, v ?? 'ask')),
        ),
        IconButton(
          key: Key('tool-remove-$i'),
          tooltip: 'Remove rule',
          icon: const Icon(Icons.close, size: 18),
          onPressed: () => setState(() => _tools.removeAt(i).$1.dispose()),
        ),
      ],
    );
  }

  @override
  Widget build(BuildContext context) => Card(
    key: const Key('agent-settings'),
    child: Padding(
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('Agent mode', style: Theme.of(context).textTheme.titleSmall),
          const SizedBox(height: 8),
          Wrap(
            spacing: 16,
            runSpacing: 8,
            children: [
              for (final (key, label, def) in _limits)
                SizedBox(
                  width: 200,
                  child: TextField(
                    key: Key('agent-$key'),
                    controller: _nums[key],
                    decoration: InputDecoration(labelText: label, hintText: '$def (default)'),
                  ),
                ),
            ],
          ),
          const SizedBox(height: 16),
          Text('Tool rules', style: Theme.of(context).textTheme.labelLarge),
          const Text(
            'Tools without a rule ask first. Notebook and run rules come before these.',
            style: TextStyle(color: Tokens.textMuted),
          ),
          for (var i = 0; i < _tools.length; i++) _toolRow(i),
          TextButton.icon(
            key: const Key('tool-add'),
            onPressed: () => setState(() => _tools.add((TextEditingController(), 'ask'))),
            icon: const Icon(Icons.add, size: 16),
            label: const Text('Add rule'),
          ),
          if (_error != null) Text(_error!, style: TextStyle(color: Theme.of(context).colorScheme.error)),
          const SizedBox(height: 8),
          Row(
            children: [
              FilledButton(key: const Key('agent-save'), onPressed: _save, child: const Text('Save')),
              const SizedBox(width: 12),
              if (_status != null) Text(_status!, style: const TextStyle(color: Tokens.textMuted)),
            ],
          ),
        ],
      ),
    ),
  );
}
