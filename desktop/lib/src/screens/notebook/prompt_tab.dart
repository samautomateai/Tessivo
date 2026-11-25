import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../../app_state.dart';
import '../../core/models.dart';
import '../../design/components/buttons.dart';
import '../../design/components/fields.dart';
import '../../design/components/tags.dart';
import '../../design/icons.dart';
import '../../design/sizes.dart';
import '../../widgets/prompt_history_dialog.dart';
import '../../widgets/prompt_menu.dart';
import 'tab_section.dart';

/// The Prompt tab: what Run asks, how it reads the sources, and the saved prompts to start from.
class PromptTab extends StatelessWidget {
  const PromptTab({
    super.key,
    required this.state,
    required this.controller,
    required this.promptName,
    required this.onUse,
    this.perSource = false,
    this.onPerSource,
  });

  final AppState state;
  final TextEditingController controller;

  /// The saved prompt in the box, if its text is unchanged.
  final String? promptName;
  final void Function(SavedPrompt prompt) onUse;

  /// Read each source on its own first (RUN-4); saved with a prompt.
  final bool perSource;
  final ValueChanged<bool>? onPerSource;

  Future<void> _history(BuildContext context, String name) async {
    final versions = await state.promptVersions(name);
    if (!context.mounted) return;
    final pick = await showPromptHistory(context, name, versions);
    if (pick == null) return;
    final restored = await state.restorePromptVersion(name, pick.version);
    if (restored != null) onUse(restored);
  }

  @override
  Widget build(BuildContext context) {
    final saved = state.prompts.where((p) => p.name == promptName).firstOrNull;
    return TabBody(
      children: [
        TabSection(
          title: 'Prompt',
          helper: 'Run sends this with the included sources and the memory.',
          actions: [
            PromptMenu(
              state: state,
              text: () => controller.text,
              chain: () => perSource ? chainPerSource : '',
              onUse: onUse,
            ),
          ],
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              TvTextField(label: '', fieldKey: const Key('prompt'), controller: controller, minLines: 6, maxLines: 18),
              const SizedBox(height: TvSpace.s8),
              Wrap(
                spacing: TvSpace.s12,
                runSpacing: TvSpace.s4,
                crossAxisAlignment: WrapCrossAlignment.center,
                children: [
                  TvCheckbox(
                    key: const Key('chain-per-source'),
                    label: 'Read each source on its own first',
                    value: perSource,
                    onChanged: onPerSource,
                  ),
                  if (promptName case final n?) TvTag('Saved prompt: $n', tone: TvTagTone.blue),
                  if (saved != null && saved.version > 0) TvTag('Version ${saved.version}'),
                  if (saved != null)
                    TvToolButton(
                      'History…',
                      key: const Key('prompt-history'),
                      icon: TvIcon.schedules,
                      onPressed: () => unawaited(_history(context, saved.name)),
                    ),
                ],
              ),
            ],
          ),
        ),
      ],
    );
  }
}
