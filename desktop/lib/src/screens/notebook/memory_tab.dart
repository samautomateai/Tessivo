import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../../app_state.dart';
import '../../core/models.dart';
import '../../design/colors.dart';
import '../../design/components/buttons.dart';
import '../../design/icons.dart';
import '../../design/sizes.dart';
import '../../design/type.dart';
import '../../widgets/text_prompt_dialog.dart';
import 'tab_section.dart';

/// The Memory tab: short facts sent with every run of this notebook.
class MemoryTab extends StatelessWidget {
  const MemoryTab({super.key, required this.state, required this.notebook});

  final AppState state;
  final Notebook notebook;

  Future<void> _add(BuildContext context) async {
    final fact = await askText(
      context,
      title: 'Remember for this notebook',
      label: 'Fact',
      maxLength: maxMemoryItemLength,
    );
    if (fact == null || fact.trim().isEmpty || !context.mounted) return;
    await guarded(context, () => state.setMemory([...notebook.memory, fact]));
  }

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    final memory = notebook.memory;
    return TabBody(
      children: [
        TabSection(
          title: 'Memory',
          helper: '${memory.length} of $maxMemoryItems facts. They go with every run.',
          actions: [
            TvButton(
              'Add',
              key: const Key('memory-add'),
              icon: TvIcon.add,
              dense: true,
              onPressed: memory.length >= maxMemoryItems ? null : () => unawaited(_add(context)),
            ),
          ],
          child: memory.isEmpty
              ? const EmptyNote('Facts the model should always know here, like naming rules or the audience.')
              : Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    for (final (i, fact) in memory.indexed)
                      Container(
                        key: Key('memory-$i'),
                        constraints: const BoxConstraints(minHeight: TvSize.tableRow),
                        padding: const EdgeInsets.only(left: TvSpace.s12, right: TvSpace.s4),
                        decoration: BoxDecoration(
                          color: c.layer01,
                          border: Border(bottom: BorderSide(color: c.borderSubtle)),
                        ),
                        child: Row(
                          children: [
                            Expanded(
                              child: Text(fact, style: TvType.body.copyWith(color: c.textPrimary)),
                            ),
                            TvIconButton(
                              TvIcon.close,
                              key: Key('memory-remove-$i'),
                              tooltip: 'Forget',
                              onPressed: () =>
                                  unawaited(guarded(context, () => state.setMemory([...memory]..removeAt(i)))),
                            ),
                          ],
                        ),
                      ),
                  ],
                ),
        ),
      ],
    );
  }
}
