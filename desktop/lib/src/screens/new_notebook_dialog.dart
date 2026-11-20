import 'package:material_ui/material_ui.dart';

import '../app_state.dart';
import '../core/core_client.dart';
import '../core/models.dart';
import '../widgets/work_type_badge.dart';

/// Step 1 of creating a notebook: pick a work type and name it.
Future<void> showNewNotebookDialog(BuildContext context, AppState state) {
  return showDialog<void>(
    context: context,
    builder: (_) => NewNotebookDialog(state: state),
  );
}

class NewNotebookDialog extends StatefulWidget {
  const NewNotebookDialog({super.key, required this.state});

  final AppState state;

  @override
  State<NewNotebookDialog> createState() => _NewNotebookDialogState();
}

class _NewNotebookDialogState extends State<NewNotebookDialog> {
  WorkTypeId _type = WorkTypeId.analyze;
  final _name = TextEditingController();
  String? _error;

  @override
  void dispose() {
    _name.dispose();
    super.dispose();
  }

  Future<void> _create() async {
    final name = _name.text.trim();
    if (name.isEmpty) {
      setState(() => _error = 'Enter a name');
      return;
    }
    final navigator = Navigator.of(context);
    await widget.state.createNotebook(name, _type);
    navigator.pop();
  }

  @override
  Widget build(BuildContext context) {
    final selected = builtInWorkTypes.firstWhere((w) => w.id == _type);
    return AlertDialog(
      title: const Text('What kind of work is this notebook for?'),
      content: SizedBox(
        width: 640,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Wrap(
              spacing: 12,
              runSpacing: 12,
              children: [for (final w in builtInWorkTypes) _typeCard(w)],
            ),
            const SizedBox(height: 16),
            Text('Suggested context: ${selected.suggestedContext.join(', ')}'),
            const SizedBox(height: 16),
            TextField(
              key: const Key('notebook-name'),
              controller: _name,
              decoration: InputDecoration(labelText: 'Notebook name', errorText: _error),
              onChanged: (_) => setState(() => _error = null),
            ),
          ],
        ),
      ),
      actions: [
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
          child: Padding(
            padding: const EdgeInsets.all(14),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                WorkTypeBadge(workType: w.id),
                const SizedBox(height: 8),
                Text(w.question),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
