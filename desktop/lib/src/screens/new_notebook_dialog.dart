import 'package:material_ui/material_ui.dart';

import '../app_state.dart';
import '../core/core_client.dart';
import '../core/models.dart';
import '../design/components/tags.dart';

/// Step 1 of creating a notebook: pick a work type and name it.
Future<void> showNewNotebookDialog(BuildContext context, AppState state, {bool research = false}) {
  return showDialog<void>(
    context: context,
    builder: (_) => NewNotebookDialog(state: state, research: research),
  );
}

class NewNotebookDialog extends StatefulWidget {
  const NewNotebookDialog({super.key, required this.state, this.research = false});

  final AppState state;

  /// Open on the Research choice (from the Research side bar).
  final bool research;

  @override
  State<NewNotebookDialog> createState() => _NewNotebookDialogState();
}

class _NewNotebookDialogState extends State<NewNotebookDialog> {
  WorkTypeId _type = WorkTypeId.analyze;
  late bool _research = widget.research;
  final _name = TextEditingController();
  final _description = TextEditingController();
  String? _error;

  @override
  void dispose() {
    _name.dispose();
    _description.dispose();
    super.dispose();
  }

  Future<void> _create() async {
    final name = _name.text.trim();
    if (name.isEmpty) {
      setState(() => _error = 'Enter a name');
      return;
    }
    final navigator = Navigator.of(context);
    if (_research) {
      await widget.state.createResearch(name, description: _description.text.trim());
    } else {
      await widget.state.createNotebook(name, _type);
    }
    navigator.pop();
  }

  Widget _kind(String key, String label, String hint, bool research) => Tooltip(
    message: hint,
    child: ChoiceChip(
      key: Key(key),
      label: Text(label),
      selected: _research == research,
      onSelected: (_) => setState(() => _research = research),
    ),
  );

  @override
  Widget build(BuildContext context) {
    final selected = builtInWorkTypes.firstWhere((w) => w.id == _type);
    return AlertDialog(
      title: Text(_research ? 'What will you research?' : 'What kind of work is this notebook for?'),
      content: SizedBox(
        width: 640,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Wrap(
              spacing: 8,
              children: [
                _kind('kind-job', 'Job', 'Run prompts over context and keep the outputs', false),
                _kind('kind-research', 'Research', 'Ask questions across sources and keep notes', true),
              ],
            ),
            const SizedBox(height: 16),
            if (!_research) ...[
              Wrap(spacing: 12, runSpacing: 12, children: [for (final w in builtInWorkTypes) _typeCard(w)]),
              const SizedBox(height: 16),
              Text('Suggested context: ${selected.suggestedContext.join(', ')}'),
              const SizedBox(height: 16),
            ],
            TextField(
              key: const Key('notebook-name'),
              controller: _name,
              decoration: InputDecoration(labelText: 'Notebook name', errorText: _error),
              onChanged: (_) => setState(() => _error = null),
            ),
            if (_research)
              TextField(
                key: const Key('research-description'),
                controller: _description,
                maxLength: 500,
                decoration: const InputDecoration(labelText: 'What is it about? (optional)'),
              ),
          ],
        ),
      ),
      actions: [
        if (!_research)
          TextButton(
            key: const Key('browse-templates'),
            onPressed: () {
              Navigator.of(context).pop();
              widget.state.go(Section.templates);
            },
            child: const Text('Start from a template'),
          ),
        TextButton(onPressed: () => Navigator.of(context).pop(), child: const Text('Cancel')),
        FilledButton(key: const Key('create-notebook'), onPressed: _create, child: const Text('Create')),
      ],
    );
  }

  Widget _typeCard(WorkType w) {
    final isSelected = w.id == _type;
    return SizedBox(
      width: 300,
      child: Card(
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(12),
          side: BorderSide(
            color: isSelected ? Theme.of(context).colorScheme.primary : Theme.of(context).dividerColor,
            width: isSelected ? 2 : 1,
          ),
        ),
        child: InkWell(
          key: Key('type-${w.id.name}'),
          borderRadius: BorderRadius.circular(12),
          onTap: () => setState(() => _type = w.id),
          child: Semantics(
            selected: isSelected,
            child: Padding(
              padding: const EdgeInsets.all(14),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [WorkTypeTag(w.id), const SizedBox(height: 8), Text(w.question)],
              ),
            ),
          ),
        ),
      ),
    );
  }
}
