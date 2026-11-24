import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../app_state.dart';
import '../design/colors.dart';
import '../design/components/buttons.dart';
import '../design/icons.dart';
import '../design/sizes.dart';
import '../design/type.dart';
import '../widgets/approvals_sheet.dart';
import '../widgets/command_palette.dart';
import '../widgets/search_box.dart';

/// Where the user is, for the header's breadcrumbs.
String breadcrumbs(AppState state) => switch (state.section) {
  Section.notebooks => state.selected == null ? 'Notebooks' : 'Notebooks / ${state.selected!.name}',
  Section.research => switch (state.researchState?.notebookId) {
    final id? => 'Research / ${state.research.where((r) => r.id == id).firstOrNull?.name ?? ''}',
    null => 'Research',
  },
  Section.chat => 'Chat',
  Section.schedules => 'Schedules',
  Section.templates => 'Templates',
  Section.library => 'Outputs',
  Section.settings => 'Settings',
};

/// The Carbon shell header: product, breadcrumbs, search with the palette
/// hint, and approvals and notifications on the right.
class TvHeader extends StatelessWidget {
  const TvHeader({super.key, required this.state});

  final AppState state;

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    return Container(
      height: TvSize.header,
      padding: const EdgeInsets.only(left: TvSpace.s12, right: TvSpace.s8),
      decoration: BoxDecoration(
        color: c.background,
        border: Border(bottom: BorderSide(color: c.borderStrong)),
      ),
      child: Row(
        children: [
          Container(
            width: 18,
            height: 18,
            decoration: BoxDecoration(color: c.buttonPrimary, borderRadius: BorderRadius.circular(TvRadius.small)),
          ),
          const SizedBox(width: TvSpace.s8),
          Text('Tessivo', style: TvType.heading01.copyWith(color: c.textPrimary)),
          Container(
            width: 1,
            height: 18,
            margin: const EdgeInsets.symmetric(horizontal: TvSpace.s12),
            color: c.borderStrong,
          ),
          Flexible(
            flex: 2,
            child: Text(
              breadcrumbs(state),
              key: const Key('breadcrumbs'),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: TvType.body.copyWith(color: c.textSecondary),
            ),
          ),
          const SizedBox(width: TvSpace.s16),
          Flexible(
            flex: 3,
            child: Center(
              child: ConstrainedBox(
                constraints: const BoxConstraints(maxWidth: 460),
                child: Row(
                  children: [
                    Expanded(child: SearchBox(state: state)),
                    const SizedBox(width: TvSpace.s4),
                    TvButton(
                      'Ctrl K',
                      key: const Key('palette-hint'),
                      dense: true,
                      kind: TvButtonKind.ghost,
                      tooltip: 'Run a command',
                      onPressed: () => unawaited(showPalette(context, state)),
                    ),
                  ],
                ),
              ),
            ),
          ),
          const Spacer(),
          TvIconButton(
            TvIcon.approvals,
            key: const Key('approvals-badge'),
            tooltip: state.approvals.isEmpty ? 'No approvals waiting' : 'Approvals waiting',
            badge: state.approvals.length,
            onPressed: () => unawaited(showApprovalsSheet(context, state)),
          ),
          TvIconButton(
            TvIcon.notifications,
            key: const Key('notifications'),
            tooltip: state.notices.isEmpty ? 'No notifications' : 'Notifications',
            badge: state.notices.length,
            onPressed: state.notices.isEmpty ? null : state.dismissNotice,
          ),
        ],
      ),
    );
  }
}
