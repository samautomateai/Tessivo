import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../app_state.dart';
import '../core/models.dart';
import '../design/colors.dart';
import '../design/components/buttons.dart';
import '../design/components/headers.dart';
import '../design/components/rows.dart';
import '../design/components/tags.dart';
import '../design/icons.dart';
import '../design/work_types.dart';
import '../screens/new_notebook_dialog.dart';
import 'layout_prefs.dart';

/// The side bar's frame: [width] wide, its right edge dragged to resize.
class SideBarFrame extends StatelessWidget {
  const SideBarFrame({
    super.key,
    required this.width,
    required this.onResize,
    required this.onResized,
    required this.child,
  });

  final double width;
  final ValueChanged<double> onResize;
  final ValueChanged<double> onResized;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    var live = width;
    return Container(
      key: const Key('side-bar'),
      width: width,
      decoration: BoxDecoration(
        color: c.layer01,
        border: Border(right: BorderSide(color: c.borderSubtle)),
      ),
      child: Stack(
        children: [
          Positioned.fill(child: child),
          Positioned(
            right: 0,
            top: 0,
            bottom: 0,
            width: 5,
            child: MouseRegion(
              cursor: SystemMouseCursors.resizeColumn,
              child: GestureDetector(
                key: const Key('side-bar-edge'),
                behavior: HitTestBehavior.translucent,
                onHorizontalDragUpdate: (d) {
                  live = (live + d.delta.dx).clamp(LayoutPrefs.minWidth, LayoutPrefs.maxWidth);
                  onResize(live);
                },
                onHorizontalDragEnd: (_) => onResized(live),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

/// Notebooks: the list (archived ones folded away) with New notebook.
class NotebooksSideBar extends StatelessWidget {
  const NotebooksSideBar({super.key, required this.state});

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
            ],
          ),
        ),
      ],
    );
  }
}

/// Research: the research notebooks with their source counts.
class ResearchSideBar extends StatelessWidget {
  const ResearchSideBar({super.key, required this.state});

  final AppState state;

  @override
  Widget build(BuildContext context) => Column(
    crossAxisAlignment: CrossAxisAlignment.stretch,
    children: [
      TvPanelHeader(
        'Research',
        actions: [
          TvIconButton(
            TvIcon.add,
            key: const Key('new-research'),
            tooltip: 'New research notebook',
            onPressed: () => unawaited(showNewNotebookDialog(context, state, research: true)),
          ),
        ],
      ),
      Expanded(
        child: ListView(
          children: [
            for (final r in state.research)
              TvTreeRow(
                key: Key('research-${r.id}'),
                leading: const TvIconView(TvIcon.research),
                label: r.name,
                trailing: r.sources.isEmpty ? null : '${r.sources.length}',
                selected: state.section == Section.research && r.id == state.researchState?.notebookId,
                onTap: () => state.selectResearch(r.id),
              ),
          ],
        ),
      ),
    ],
  );
}
