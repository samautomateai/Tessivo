import 'package:material_ui/material_ui.dart';

import '../app_state.dart';
import '../chat_state.dart';
import '../core/core_client.dart';
import '../core/models.dart';
import '../theme.dart';
import '../widgets/text_prompt_dialog.dart';

/// Asks which work type to save a reply as, then saves it to the library.
Future<void> showSaveReplyDialog(BuildContext context, ChatState chat, ChatMessage reply) async {
  final messenger = ScaffoldMessenger.of(context);
  final type = await showDialog<WorkTypeId>(context: context, builder: (_) => const _WorkTypeDialog());
  if (type == null) return;
  messenger.showSnackBar(const SnackBar(content: Text('Structuring the reply…')));
  final out = await chat.saveReply(reply.id, type);
  messenger.hideCurrentSnackBar();
  if (out != null) {
    messenger.showSnackBar(
      SnackBar(key: const Key('saved-output'), content: Text('Saved "${out.title}" to the outputs library')),
    );
  }
}

class _WorkTypeDialog extends StatefulWidget {
  const _WorkTypeDialog();

  @override
  State<_WorkTypeDialog> createState() => _WorkTypeDialogState();
}

class _WorkTypeDialogState extends State<_WorkTypeDialog> {
  WorkTypeId _type = WorkTypeId.summarize;

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: const Text('Save reply as'),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SegmentedButton<WorkTypeId>(
            key: const Key('save-type'),
            segments: [for (final w in WorkTypeId.values) ButtonSegment(value: w, label: Text(workTypeLabel(w)))],
            selected: {_type},
            onSelectionChanged: (s) => setState(() => _type = s.first),
          ),
          const SizedBox(height: 12),
          const Text(
            "The model restructures the reply into this work type's format; it is checked before saving.",
            style: TextStyle(color: Tokens.textMuted),
          ),
        ],
      ),
      actions: [
        TextButton(onPressed: () => Navigator.of(context).pop(), child: const Text('Cancel')),
        FilledButton(
          key: const Key('save-confirm'),
          onPressed: () => Navigator.of(context).pop(_type),
          child: const Text('Save'),
        ),
      ],
    );
  }
}

/// Shows what "Make this repeatable" will create, lets the user adjust it,
/// creates the notebook and opens it.
Future<void> showRepeatDialog(BuildContext context, AppState state) async {
  final plan = await state.chat.plan();
  if (plan == null || !context.mounted) return;
  final nb = await showDialog<Notebook>(
    context: context,
    builder: (_) => _RepeatDialog(state: state, plan: plan),
  );
  if (nb != null) await state.openNotebook(nb.id);
}

class _RepeatDialog extends StatefulWidget {
  const _RepeatDialog({required this.state, required this.plan});

  final AppState state;
  final RepeatablePlan plan;

  @override
  State<_RepeatDialog> createState() => _RepeatDialogState();
}

class _RepeatDialogState extends State<_RepeatDialog> {
  late final _name = TextEditingController(text: widget.plan.name);
  late WorkTypeId _type = widget.plan.workType;
  late bool _useTemplate = widget.plan.templateId != null;
  bool _busy = false;

  @override
  void dispose() {
    _name.dispose();
    super.dispose();
  }

  bool get _templateApplies => widget.plan.templateId != null && _type == widget.plan.workType;

  Future<void> _create() async {
    setState(() => _busy = true);
    final nb = await widget.state.chat.makeRepeatable(
      name: _name.text.trim().isEmpty ? null : _name.text.trim(),
      workType: _type,
      templateId: _templateApplies && _useTemplate ? widget.plan.templateId : 'none',
    );
    if (!mounted) return;
    setState(() => _busy = false);
    if (nb != null) Navigator.of(context).pop(nb);
  }

  @override
  Widget build(BuildContext context) {
    final p = widget.plan;
    final error = widget.state.chat.error;
    return AlertDialog(
      title: const Text('Make this repeatable'),
      content: SizedBox(
        width: 560,
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              TextField(
                key: const Key('repeat-name'),
                controller: _name,
                decoration: const InputDecoration(labelText: 'Notebook name'),
              ),
              const SizedBox(height: 12),
              DropdownButton<WorkTypeId>(
                key: const Key('repeat-type'),
                value: _type,
                items: [for (final w in WorkTypeId.values) DropdownMenuItem(value: w, child: Text(workTypeLabel(w)))],
                onChanged: (w) => setState(() => _type = w ?? _type),
              ),
              if (_templateApplies)
                CheckboxListTile(
                  key: const Key('repeat-template'),
                  contentPadding: EdgeInsets.zero,
                  value: _useTemplate,
                  onChanged: (v) => setState(() => _useTemplate = v ?? false),
                  title: Text('Start from the "${p.templateName ?? p.templateId}" template'),
                ),
              const SizedBox(height: 8),
              const Text('Prompt', style: TextStyle(fontWeight: FontWeight.w600)),
              Text(p.prompt, maxLines: 6, overflow: TextOverflow.ellipsis),
              const SizedBox(height: 8),
              Text(
                p.context.isEmpty ? 'No context to carry over.' : 'Context: ${p.context.length} item(s)',
                style: const TextStyle(fontWeight: FontWeight.w600),
              ),
              for (final c in p.context) Text(c, style: const TextStyle(fontSize: 12)),
              for (final s in p.skipped)
                Text('Skipped: $s', style: const TextStyle(color: Tokens.textMuted, fontSize: 12)),
              if (error != null) ...[
                const SizedBox(height: 8),
                Text(
                  error,
                  key: const Key('repeat-error'),
                  style: const TextStyle(color: Colors.red),
                ),
              ],
            ],
          ),
        ),
      ),
      actions: [
        TextButton(onPressed: () => Navigator.of(context).pop(), child: const Text('Cancel')),
        FilledButton(
          key: const Key('repeat-create'),
          onPressed: _busy ? null : _create,
          child: const Text('Create notebook'),
        ),
      ],
    );
  }
}

/// Global chat memory: facts every chat sees.
Future<void> showChatMemoryDialog(BuildContext context, ChatState chat) {
  return showDialog<void>(
    context: context,
    builder: (_) => _MemoryDialog(chat: chat),
  );
}

class _MemoryDialog extends StatelessWidget {
  const _MemoryDialog({required this.chat});

  final ChatState chat;

  Future<void> _add(BuildContext context) async {
    final fact = await askText(context, title: 'Remember in every chat', label: 'Fact', maxLength: maxMemoryItemLength);
    if (fact != null) await chat.setMemory([...chat.memory, fact]);
  }

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: chat,
      builder: (context, _) => AlertDialog(
        title: const Text('Chat memory'),
        content: SizedBox(
          width: 520,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const Text(
                'Facts every chat should know, like your role or preferences. Separate from notebook memory.',
                style: TextStyle(color: Tokens.textMuted),
              ),
              const SizedBox(height: 8),
              if (chat.memory.isEmpty) const Text('Nothing yet.'),
              for (final (i, fact) in chat.memory.indexed)
                ListTile(
                  key: Key('chat-memory-$i'),
                  dense: true,
                  contentPadding: EdgeInsets.zero,
                  title: Text(fact),
                  trailing: IconButton(
                    key: Key('chat-memory-remove-$i'),
                    tooltip: 'Forget',
                    icon: const Icon(Icons.close, size: 18),
                    onPressed: () => chat.setMemory([...chat.memory]..removeAt(i)),
                  ),
                ),
              if (chat.error != null) Text(chat.error!, style: const TextStyle(color: Colors.red)),
            ],
          ),
        ),
        actions: [
          TextButton.icon(
            key: const Key('chat-memory-add'),
            onPressed: chat.memory.length >= maxMemoryItems ? null : () => _add(context),
            icon: const Icon(Icons.add, size: 18),
            label: const Text('Add fact'),
          ),
          TextButton(onPressed: () => Navigator.of(context).pop(), child: const Text('Done')),
        ],
      ),
    );
  }
}
