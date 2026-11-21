import 'package:material_ui/material_ui.dart';

import '../app_state.dart';
import '../core/models.dart';
import '../theme.dart';
import '../widgets/work_type_badge.dart';

/// Template gallery: browse built-in and user templates, start a notebook from
/// one, save the selected notebook as a template, and remove user templates.
class TemplatesScreen extends StatefulWidget {
  const TemplatesScreen({super.key, required this.state});

  final AppState state;

  @override
  State<TemplatesScreen> createState() => _TemplatesScreenState();
}

class _TemplatesScreenState extends State<TemplatesScreen> {
  WorkTypeId? _filter;
  List<TemplateInfo> _items = const [];
  String? _error;
  int _request = 0;

  @override
  void initState() {
    super.initState();
    _reload();
  }

  Future<void> _reload() async {
    final req = ++_request;
    try {
      final items = await widget.state.core.listTemplates(workType: _filter);
      if (mounted && req == _request) setState(() => _items = items);
    } on Object catch (e) {
      if (mounted) setState(() => _error = '$e');
    }
  }

  Future<void> _run(Future<void> Function() action) async {
    setState(() => _error = null);
    try {
      await action();
    } on Object catch (e) {
      if (mounted) setState(() => _error = e is FormatException ? e.message : '$e');
    }
  }

  Future<void> _use(TemplateInfo t) async {
    final name = await _askName(context, title: 'New notebook from "${t.name}"', initial: t.name);
    if (name == null) return;
    await _run(() => widget.state.createFromTemplate(t.id, name: name));
  }

  Future<void> _save() async {
    final nb = widget.state.selected;
    if (nb == null) return;
    final name = await _askName(context, title: 'Save "${nb.name}" as a template', initial: nb.name);
    if (name == null) return;
    await _run(() async {
      await widget.state.core.saveAsTemplate(nb.id, name: name);
      await _reload();
    });
  }

  Future<void> _delete(TemplateInfo t) => _run(() async {
    await widget.state.core.deleteTemplate(t.id);
    await _reload();
  });

  @override
  Widget build(BuildContext context) {
    final text = Theme.of(context).textTheme;
    final nb = widget.state.selected;
    return ListView(
      padding: const EdgeInsets.all(28),
      children: [
        Row(
          children: [
            Expanded(
              child: Text('Templates', style: text.headlineSmall?.copyWith(fontWeight: FontWeight.w600)),
            ),
            OutlinedButton.icon(
              key: const Key('save-as-template'),
              onPressed: nb == null ? null : _save,
              icon: const Icon(Icons.bookmark_add_outlined, size: 18),
              label: Text(nb == null ? 'Save notebook as template' : 'Save "${nb.name}" as template'),
            ),
          ],
        ),
        const SizedBox(height: 4),
        const Text(
          'Templates carry prompts, context slots and memory. Never your files or secrets.',
          style: TextStyle(color: Tokens.textMuted),
        ),
        const SizedBox(height: 16),
        Wrap(spacing: 8, children: [_chip(null, 'All'), for (final w in WorkTypeId.values) _chip(w, workTypeLabel(w))]),
        if (_error != null) ...[
          const SizedBox(height: 12),
          Text(_error!, key: const Key('template-error'), style: const TextStyle(color: Colors.red)),
        ],
        const SizedBox(height: 16),
        if (_items.isEmpty) const Text('No templates for this work type yet.'),
        Wrap(
          spacing: 16,
          runSpacing: 16,
          children: [for (final t in _items) _TemplateCard(template: t, onUse: _use, onDelete: _delete)],
        ),
      ],
    );
  }

  Widget _chip(WorkTypeId? w, String label) => ChoiceChip(
    key: Key('template-filter-${w?.name ?? 'all'}'),
    label: Text(label),
    selected: _filter == w,
    onSelected: (_) {
      setState(() => _filter = w);
      _reload();
    },
  );
}

class _TemplateCard extends StatelessWidget {
  const _TemplateCard({required this.template, required this.onUse, required this.onDelete});

  final TemplateInfo template;
  final void Function(TemplateInfo) onUse;
  final void Function(TemplateInfo) onDelete;

  @override
  Widget build(BuildContext context) {
    final t = template;
    final slots = t.slots.map((s) => s.required ? '${s.name}*' : s.name).join(', ');
    final meta = [
      '${t.prompts.length} prompt${t.prompts.length == 1 ? '' : 's'}',
      if (t.scheduleCron != null) 'schedule ${t.scheduleCron}',
      if (t.isUser) 'v${t.version}',
      if (t.parent != null) 'extends ${t.parent}',
    ].join(' · ');
    return SizedBox(
      width: 320,
      child: Card(
        key: Key('template-card-${t.id}'),
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  WorkTypeBadge(workType: t.workType),
                  const Spacer(),
                  Text(t.isUser ? 'Yours' : 'Built-in', style: const TextStyle(color: Tokens.textMuted, fontSize: 12)),
                ],
              ),
              const SizedBox(height: 10),
              Text(t.name, style: Theme.of(context).textTheme.titleMedium),
              const SizedBox(height: 4),
              Text(t.description, maxLines: 3, overflow: TextOverflow.ellipsis),
              const SizedBox(height: 8),
              if (slots.isNotEmpty) Text('Context: $slots', style: const TextStyle(fontSize: 12)),
              Text(meta, style: const TextStyle(color: Tokens.textMuted, fontSize: 12)),
              const SizedBox(height: 12),
              Row(
                children: [
                  FilledButton(
                    key: Key('template-use-${t.id}'),
                    onPressed: () => onUse(t),
                    child: const Text('Use template'),
                  ),
                  const Spacer(),
                  if (t.isUser)
                    IconButton(
                      key: Key('template-delete-${t.id}'),
                      tooltip: 'Remove template',
                      icon: const Icon(Icons.delete_outline),
                      onPressed: () => onDelete(t),
                    ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// Asks for a name; returns null on cancel or when left empty.
Future<String?> _askName(BuildContext context, {required String title, required String initial}) async {
  final name = await showDialog<String>(
    context: context,
    builder: (_) => _NameDialog(title: title, initial: initial),
  );
  final trimmed = name?.trim();
  return trimmed == null || trimmed.isEmpty ? null : trimmed;
}

class _NameDialog extends StatefulWidget {
  const _NameDialog({required this.title, required this.initial});

  final String title;
  final String initial;

  @override
  State<_NameDialog> createState() => _NameDialogState();
}

class _NameDialogState extends State<_NameDialog> {
  late final _controller = TextEditingController(text: widget.initial);

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: Text(widget.title),
      content: TextField(
        key: const Key('template-name'),
        controller: _controller,
        autofocus: true,
        maxLength: 120,
        decoration: const InputDecoration(labelText: 'Name'),
        onSubmitted: (v) => Navigator.of(context).pop(v),
      ),
      actions: [
        TextButton(onPressed: () => Navigator.of(context).pop(), child: const Text('Cancel')),
        FilledButton(
          key: const Key('template-name-ok'),
          onPressed: () => Navigator.of(context).pop(_controller.text),
          child: const Text('OK'),
        ),
      ],
    );
  }
}
