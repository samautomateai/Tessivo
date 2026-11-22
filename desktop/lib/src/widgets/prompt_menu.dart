import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../app_state.dart';
import '../core/models.dart';
import 'text_prompt_dialog.dart';

/// The open notebook's saved prompts: use one, save the current text as one, delete one.
class PromptMenu extends StatelessWidget {
  const PromptMenu({super.key, required this.state, required this.text, required this.onUse});

  final AppState state;

  /// The prompt box's current text (what "Save as prompt" saves).
  final String Function() text;

  final void Function(SavedPrompt prompt) onUse;

  Future<void> _save(BuildContext context) async {
    final name = await askText(context, title: 'Save as prompt', label: 'Prompt name', maxLength: 64);
    if (name != null && name.trim().isNotEmpty) await state.savePromptAs(name.trim(), text());
  }

  @override
  Widget build(BuildContext context) {
    return PopupMenuButton<String>(
      key: const Key('prompts-menu'),
      tooltip: 'Saved prompts',
      icon: const Icon(Icons.bookmarks_outlined),
      onSelected: (v) {
        if (v == '+save') {
          unawaited(_save(context));
        } else if (v.startsWith('-')) {
          unawaited(state.deletePrompt(v.substring(1)));
        } else {
          onUse(state.prompts.firstWhere((p) => p.name == v));
        }
      },
      itemBuilder: (_) => [
        for (final p in state.prompts)
          PopupMenuItem(
            key: Key('prompt-use-${p.name}'),
            value: p.name,
            child: Text(p.model.isEmpty ? p.name : '${p.name}  ·  ${p.model}'),
          ),
        if (state.prompts.isNotEmpty) const PopupMenuDivider(),
        const PopupMenuItem(key: Key('prompt-save'), value: '+save', child: Text('Save as prompt…')),
        for (final p in state.prompts)
          PopupMenuItem(key: Key('prompt-delete-${p.name}'), value: '-${p.name}', child: Text('Delete ${p.name}')),
      ],
    );
  }
}
