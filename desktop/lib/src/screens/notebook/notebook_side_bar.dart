import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../../app_state.dart';
import '../../core/models.dart';
import '../../design/colors.dart';
import '../../design/components/buttons.dart';
import '../../design/components/headers.dart';
import '../../design/components/rows.dart';
import '../../design/components/tags.dart';
import '../../design/icons.dart';
import '../../design/work_types.dart';
import '../new_notebook_dialog.dart';

/// "v2 ← v1" for a revision whose parent is listed, else "vN".
String lineage(OutputSummary o, List<OutputSummary> all) {
  final parent = all.where((p) => p.id == o.parentId).firstOrNull;
  return parent == null ? 'v${o.version}' : 'v${o.version} ← v${parent.version}';
}

/// The Notebooks side bar: the notebooks (archived ones folded away), then
/// the open notebook's outputs, newest first.
class NotebookSideBar extends StatelessWidget {
  const NotebookSideBar({super.key, required this.state});

  final AppState state;

  Widget _row(Notebook nb) {
    final tag = workTypeLabel(nb.workType);
    return Semantics(
      key: Key('nav-${nb.id}'),
      button: true,
      selected: nb.id == state.selectedId,
      label: '${nb.name}, $tag notebook${nb.schedule == null ? '' : ', runs ${nb.schedule}'}',
      excludeSemantics: true,
      onTap: () => state.select(nb.id),
      child: Builder(
        builder: (context) => TvTreeRow(
          leading: TvStatusDot(TvColors.of(context).tag(nb.workType).dot, square: true),
          label: nb.name,
          trailing: nb.schedule,
          selected: nb.id == state.selectedId,
          onTap: () => state.select(nb.id),
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final archived = state.notebooks.where((n) => n.archived).toList();
    final outputs = state.selected == null ? const <OutputSummary>[] : state.outputs;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        TvPanelHeader(
          'Notebooks',
          actions: [
            TvIconButton(
              TvIcon.add,
              key: const Key('new-notebook'),
              tooltip: 'New notebook (Ctrl N)',
              onPressed: () => unawaited(showNewNotebookDialog(context, state)),
            ),
          ],
        ),
        Expanded(
          child: ListView(
            children: [
              for (final nb in state.notebooks)
                if (!nb.archived) _row(nb),
              if (archived.isNotEmpty)
                TvTreeRow(
                  key: const Key('archived-toggle'),
                  leading: Transform.rotate(
                    angle: state.showArchived ? 1.5708 : 0,
                    child: const TvIconView(TvIcon.expand),
                  ),
                  label: 'Archived (${archived.length})',
                  muted: true,
                  onTap: state.toggleArchived,
                ),
              if (state.showArchived)
                for (final nb in archived) _row(nb),
              if (outputs.isNotEmpty) ...[
                const TvPanelHeader('Outputs', key: Key('outputs-list')),
                for (final o in outputs)
                  TvTreeRow(
                    key: Key('output-${o.id}'),
                    leading: TvIconView(o.trigger == 'Scheduled' ? TvIcon.schedules : TvIcon.document, size: 14),
                    label: o.title,
                    trailing: lineage(o, outputs),
                    selected: o.id == state.selectedOutputId,
                    onTap: () => state.selectOutput(o.id),
                  ),
              ],
            ],
          ),
        ),
      ],
    );
  }
}
