import 'package:material_ui/material_ui.dart';

import '../app_state.dart';
import '../design/colors.dart';
import '../design/icons.dart';
import '../design/sizes.dart';
import '../design/type.dart';

/// The sections the activity bar opens, in order; Settings sits at the bottom.
const activitySections = [
  (Section.notebooks, TvIcon.notebooks, 'Notebooks'),
  (Section.research, TvIcon.research, 'Research'),
  (Section.chat, TvIcon.chat, 'Chat'),
  (Section.schedules, TvIcon.schedules, 'Schedules'),
  (Section.templates, TvIcon.templates, 'Templates'),
  (Section.library, TvIcon.document, 'Outputs'),
];

/// VS Code's activity bar: an icon with its label under it for each section.
class ActivityBar extends StatelessWidget {
  const ActivityBar({super.key, required this.state});

  final AppState state;

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    return Container(
      key: const Key('activity-bar'),
      width: TvSize.activityBar,
      decoration: BoxDecoration(
        color: c.activityBar,
        border: Border(right: BorderSide(color: c.borderSubtle)),
      ),
      child: Column(
        children: [
          // Scrolls when the window is too short (large zoom), Settings stays at the bottom.
          Expanded(
            child: SingleChildScrollView(
              child: Column(
                children: [
                  for (final (s, icon, label) in activitySections)
                    _Item(state: state, section: s, icon: icon, label: label),
                ],
              ),
            ),
          ),
          _Item(state: state, section: Section.settings, icon: TvIcon.settings, label: 'Settings'),
          const SizedBox(height: TvSpace.s8),
        ],
      ),
    );
  }
}

class _Item extends StatelessWidget {
  const _Item({required this.state, required this.section, required this.icon, required this.label});

  final AppState state;
  final Section section;
  final TvIcon icon;
  final String label;

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    final on = state.section == section;
    final color = on ? c.textPrimary : c.textHelper;
    return Semantics(
      button: true,
      selected: on,
      label: label,
      excludeSemantics: true,
      onTap: () => state.go(section),
      child: InkWell(
        key: Key('activity-${section.name}'),
        onTap: () => state.go(section),
        hoverColor: c.layer01,
        child: Container(
          height: 54,
          decoration: BoxDecoration(
            border: Border(left: BorderSide(color: on ? c.interactive : c.interactive.withValues(alpha: 0), width: 2)),
          ),
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              TvIconView(icon, size: TvSize.iconLarge, color: color),
              const SizedBox(height: TvSpace.s4),
              Text(
                label,
                maxLines: 1,
                overflow: TextOverflow.clip,
                style: TvType.caption.copyWith(color: color),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
