import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../app_state.dart';
import '../design/colors.dart';
import '../design/components/buttons.dart';
import '../design/components/headers.dart';
import '../design/components/rows.dart';
import '../design/icons.dart';
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
