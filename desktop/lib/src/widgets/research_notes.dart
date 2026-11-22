import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../core/core_client.dart';
import '../research_state.dart';
import '../theme.dart';

/// The notes column: your notes and saved answers, each in or out of the context.
class ResearchNotes extends StatelessWidget {
  const ResearchNotes({super.key, required this.state});

  final ResearchState state;

  @override
  Widget build(BuildContext context) {
    return Column(
      key: const Key('research-notes'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 16, 8, 8),
          child: Row(
            children: [
              const Expanded(
                child: Text('Notes', style: TextStyle(fontWeight: FontWeight.w600)),
              ),
              TextButton.icon(
                key: const Key('research-add-note'),
                onPressed: () => unawaited(_edit(context, state, null)),
                icon: const Icon(Icons.add, size: 16),
                label: const Text('Add'),
              ),
            ],
          ),
        ),
        Expanded(
          child: state.notes.isEmpty
              ? const Padding(
                  padding: EdgeInsets.all(16),
                  child: Text(
                    'Write a note, or save an answer to keep it. Notes you include are sent with each question.',
                    style: TextStyle(color: Tokens.textMuted),
                  ),
                )
              : ListView(
                  padding: const EdgeInsets.symmetric(horizontal: 8),
                  children: [for (final n in state.notes) _NoteCard(state: state, note: n)],
                ),
        ),
      ],
    );
  }
}

class _NoteCard extends StatelessWidget {
  const _NoteCard({required this.state, required this.note});

  final ResearchState state;
  final ResearchNote note;

  @override
  Widget build(BuildContext context) {
    final n = note;
    return Card(
      key: Key('note-${n.id}'),
      margin: const EdgeInsets.only(bottom: 8),
      child: InkWell(
        onTap: () => unawaited(_edit(context, state, n)),
        child: Padding(
          padding: const EdgeInsets.fromLTRB(12, 8, 4, 10),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  if (n.isAi) ...[
                    Container(
                      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 1),
                      decoration: BoxDecoration(
                        color: Tokens.accent.withValues(alpha: 0.12),
                        borderRadius: BorderRadius.circular(4),
                      ),
                      child: const Text('AI', style: TextStyle(color: Tokens.accent, fontSize: 11)),
                    ),
                    const SizedBox(width: 6),
                  ],
                  Expanded(
                    child: Text(
                      n.title,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(fontWeight: FontWeight.w500),
                    ),
                  ),
                  Tooltip(
                    message: n.include ? 'Sent with each question' : 'Not sent',
                    child: Checkbox(
                      key: Key('note-include-${n.id}'),
                      value: n.include,
                      onChanged: (v) => unawaited(state.setNoteIncluded(n, v ?? false)),
                    ),
                  ),
                  IconButton(
                    key: Key('note-delete-${n.id}'),
                    tooltip: 'Delete',
                    icon: const Icon(Icons.delete_outline, size: 18),
                    onPressed: () => unawaited(state.deleteNote(n.id)),
                  ),
                ],
              ),
              Text(
                n.text,
                maxLines: 4,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(color: Tokens.textMuted, fontSize: 12),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// Creates ([note] null) or edits a note.
Future<void> _edit(BuildContext context, ResearchState state, ResearchNote? note) async {
  final saved = await showDialog<ResearchNote>(
    context: context,
    builder: (_) => _NoteDialog(note: note),
  );
  if (saved != null) await state.saveNote(saved);
}

class _NoteDialog extends StatefulWidget {
  const _NoteDialog({this.note});

  final ResearchNote? note;

  @override
  State<_NoteDialog> createState() => _NoteDialogState();
}

class _NoteDialogState extends State<_NoteDialog> {
  late final _title = TextEditingController(text: widget.note?.title ?? '');
  late final _text = TextEditingController(text: widget.note?.text ?? '');

  @override
  void dispose() {
    _title.dispose();
    _text.dispose();
    super.dispose();
  }

  void _save() {
    if (_text.text.trim().isEmpty) return;
    final n = widget.note;
    final title = _title.text.trim();
    Navigator.of(context).pop(
      ResearchNote(
        id: n?.id ?? '',
        title: title.isEmpty ? _text.text.trim().split('\n').first : title,
        text: _text.text,
        origin: n?.origin ?? 'human',
        include: n?.include ?? true,
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: Text(widget.note == null ? 'New note' : 'Edit note'),
      content: SizedBox(
        width: 560,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            TextField(
              key: const Key('note-title'),
              controller: _title,
              maxLength: 200,
              decoration: const InputDecoration(labelText: 'Title'),
            ),
            TextField(
              key: const Key('note-text'),
              controller: _text,
              minLines: 4,
              maxLines: 12,
              maxLength: 64 * 1024,
              decoration: const InputDecoration(labelText: 'Note', border: OutlineInputBorder()),
            ),
          ],
        ),
      ),
      actions: [
        TextButton(onPressed: () => Navigator.of(context).pop(), child: const Text('Cancel')),
        FilledButton(key: const Key('note-save'), onPressed: _save, child: const Text('Save')),
      ],
    );
  }
}
