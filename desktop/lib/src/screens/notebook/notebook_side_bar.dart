import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../../app_state.dart';
import '../../core/models.dart';
import '../../design/colors.dart';
import '../../design/components/buttons.dart';
import '../../design/components/fields.dart';
import '../../design/components/headers.dart';
import '../../design/components/rows.dart';
import '../../design/components/tags.dart';
import '../../design/icons.dart';
import '../../design/sizes.dart';
import '../../design/type.dart';
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
      label:
          '${nb.name}, $tag notebook${nb.pinned ? ', pinned' : ''}'
          '${nb.tags.isEmpty ? '' : ', tagged ${nb.tags.join(', ')}'}'
          '${nb.schedule == null ? '' : ', runs ${nb.schedule}'}',
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
    final c = TvColors.of(context);
    final archived = state.notebooks.where((n) => n.archived).toList();
    final visible = state.notebooks.where((n) => !n.archived && state.matchesTagFilter(n)).toList();
    final pinned = visible.where((n) => n.pinned).toList();
    final tagged = state.notebooks.any((n) => n.tags.isNotEmpty);
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
        if (tagged || state.tagFilter.isNotEmpty)
          Padding(
            padding: const EdgeInsets.fromLTRB(TvSpace.s12, 0, TvSpace.s12, TvSpace.s8),
            child: TextField(
              key: const Key('tag-filter'),
              style: TvType.body.copyWith(color: c.textPrimary),
              cursorColor: c.interactive,
              decoration: tvFieldDecoration(c, hint: 'Filter by tag'),
              onChanged: state.setTagFilter,
            ),
          ),
        Expanded(
          child: ListView(
            children: [
              if (pinned.isNotEmpty) ...[
                const _GroupLabel('PINNED'),
                for (final nb in pinned) _row(nb),
                const _GroupLabel('NOTEBOOKS'),
              ],
              for (final nb in visible)
                if (!nb.pinned) _row(nb),
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

class _GroupLabel extends StatelessWidget {
  const _GroupLabel(this.text);

  final String text;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.fromLTRB(TvSpace.s16, TvSpace.s8, TvSpace.s16, TvSpace.s4),
    child: Text(text, style: TvType.caption.copyWith(color: TvColors.of(context).textHelper)),
  );
}
