import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../app_state.dart';
import '../core/core_client.dart';
import '../core/models.dart';
import '../design/components/menu.dart';
import '../design/icons.dart';
import 'prompt_library_dialog.dart';
import 'text_prompt_dialog.dart';

/// The open notebook's saved prompts: use one, save the current text as one, delete one.
class PromptMenu extends StatelessWidget {
  const PromptMenu({super.key, required this.state, required this.text, required this.onUse, this.chain});

  final AppState state;

  /// The prompt box's current text (what "Save as prompt" saves).
  final String Function() text;

  /// The chain setting saved with the prompt ('' or per_source).
  final String Function()? chain;

  final void Function(SavedPrompt prompt) onUse;

  Future<void> _save(BuildContext context) async {
    final name = await askText(context, title: 'Save as prompt', label: 'Prompt name', maxLength: 64);
    if (name != null && name.trim().isNotEmpty) {
      await state.savePromptAs(name.trim(), text(), chain: chain?.call() ?? '');
    }
  }

  /// Fills the prompt box from the starter library.
  Future<void> _library(BuildContext context) async {
    final p = await showDialog<LibraryPrompt>(
      context: context,
      builder: (_) =>
          PromptLibraryDialog(prompts: state.core.promptLibrary(), workType: state.selected?.workType.name ?? ''),
    );
    if (p != null) onUse(SavedPrompt(name: p.name, text: p.text));
  }

  @override
  Widget build(BuildContext context) {
    return TvMenuButton(
      key: const Key('prompts-menu'),
      label: 'Saved prompts',
      icon: TvIcon.document,
      items: [
        for (final p in state.prompts)
          TvMenuItem(
            p.model.isEmpty ? p.name : '${p.name}  ·  ${p.model}',
            key: Key('prompt-use-${p.name}'),
            onSelected: () => onUse(p),
          ),
        if (state.prompts.isNotEmpty) const TvMenuItem.divider(),
        TvMenuItem(
          'From the library…',
          key: const Key('prompt-library'),
          onSelected: () => unawaited(_library(context)),
        ),
        TvMenuItem('Save as prompt…', key: const Key('prompt-save'), onSelected: () => unawaited(_save(context))),
        for (final p in state.prompts)
          TvMenuItem(
            'Delete ${p.name}',
            key: Key('prompt-delete-${p.name}'),
            onSelected: () => unawaited(state.deletePrompt(p.name)),
          ),
      ],
    );
  }
}
