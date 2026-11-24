import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../core/core_client.dart';
import '../design/colors.dart';
import '../design/components/buttons.dart';
import '../design/components/tags.dart';
import '../design/icons.dart';
import '../design/sizes.dart';
import '../design/type.dart';
import '../research_state.dart';

/// The Notes tab: your notes and saved answers, each in or out of the context.
class ResearchNotes extends StatelessWidget {
  const ResearchNotes({super.key, required this.state});

  final ResearchState state;

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    return ListView(
      key: const Key('research-notes'),
      padding: const EdgeInsets.fromLTRB(TvSpace.s24, TvSpace.s16, TvSpace.s24, TvSpace.s24),
      children: [
        Row(
          children: [
            Expanded(
              child: Text(
                'Notes you include are sent with each question.',
                style: TvType.helper.copyWith(color: c.textHelper),
              ),
            ),
            TvButton(
              'Add',
              key: const Key('research-add-note'),
              icon: TvIcon.add,
              dense: true,
              onPressed: () => unawaited(_edit(context, state, null)),
            ),
          ],
        ),
        const SizedBox(height: TvSpace.s12),
        if (state.notes.isEmpty)
          Text('Write a note, or save an answer to keep it.', style: TvType.body.copyWith(color: c.textSecondary)),
        for (final n in state.notes) _NoteCard(state: state, note: n),
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
    final c = TvColors.of(context);
    final n = note;
    return Material(
      key: Key('note-${n.id}'),
      color: c.layer01,
      child: InkWell(
        onTap: () => unawaited(_edit(context, state, n)),
        hoverColor: c.layer02,
        child: Container(
          padding: const EdgeInsets.fromLTRB(TvSpace.s12, TvSpace.s8, TvSpace.s4, TvSpace.s8),
          decoration: BoxDecoration(
            border: Border(bottom: BorderSide(color: c.borderSubtle)),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  if (n.isAi) ...[const TvTag('AI', tone: TvTagTone.blue), const SizedBox(width: TvSpace.s8)],
                  Expanded(
                    child: Text(
                      n.title,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TvType.body.copyWith(color: c.textPrimary, fontWeight: FontWeight.w500),
                    ),
                  ),
                  Tooltip(
                    message: n.include ? 'Sent with each question' : 'Not sent',
                    child: Checkbox(
                      key: Key('note-include-${n.id}'),
                      value: n.include,
                      visualDensity: VisualDensity.compact,
                      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(TvRadius.small)),
                      onChanged: (v) => unawaited(state.setNoteIncluded(n, v ?? false)),
                    ),
                  ),
                  TvIconButton(
                    TvIcon.close,
                    key: Key('note-delete-${n.id}'),
                    tooltip: 'Delete',
                    onPressed: () => unawaited(state.deleteNote(n.id)),
                  ),
                ],
              ),
              Text(
                n.text,
                maxLines: 4,
                overflow: TextOverflow.ellipsis,
                style: TvType.helper.copyWith(color: c.textSecondary),
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
