import 'package:material_ui/material_ui.dart';

import '../../app_state.dart';
import '../../core/models.dart';
import '../../design/components/fields.dart';
import '../../design/components/tags.dart';
import '../../widgets/prompt_menu.dart';
import 'tab_section.dart';

/// The Prompt tab: what Run asks, and the saved prompts to start from.
class PromptTab extends StatelessWidget {
  const PromptTab({
    super.key,
    required this.state,
    required this.controller,
    required this.promptName,
    required this.onUse,
  });

  final AppState state;
  final TextEditingController controller;

  /// The saved prompt in the box, if its text is unchanged.
  final String? promptName;
  final void Function(SavedPrompt prompt) onUse;

  @override
  Widget build(BuildContext context) => TabBody(
    children: [
      TabSection(
        title: 'Prompt',
        helper: 'Run sends this with the included sources and the memory.',
        actions: [
          if (promptName case final n?) TvTag('Saved prompt: $n', tone: TvTagTone.blue),
          PromptMenu(state: state, text: () => controller.text, onUse: onUse),
        ],
        child: TvTextField(label: '', fieldKey: const Key('prompt'), controller: controller, minLines: 6, maxLines: 18),
      ),
    ],
  );
}
