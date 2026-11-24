import 'package:material_ui/material_ui.dart';

import '../app_state.dart';
import '../design/components/buttons.dart';
import '../design/components/notices.dart';

/// The notices under the header: lost connection, first-run setup and the
/// newest core event.
class WorkbenchBanners extends StatelessWidget {
  const WorkbenchBanners({super.key, required this.state});

  final AppState state;

  @override
  Widget build(BuildContext context) {
    final extra = state.notices.length - 1;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        if (state.connectionLost)
          const TvInlineNotice(
            key: Key('reconnecting'),
            kind: TvNoticeKind.warning,
            title: 'Reconnecting to the Tessivo core…',
            message: 'Your work is safe; the app retries on its own.',
          ),
        if (state.setupMessage case final m? when state.section != Section.notebooks)
          TvInlineNotice(
            key: const Key('setup-banner'),
            kind: TvNoticeKind.info,
            title: 'Set up a model',
            message: m,
            action: TvButton(
              'Open settings',
              key: const Key('setup-open'),
              dense: true,
              kind: TvButtonKind.primary,
              onPressed: () => state.openSettings(SettingsSection.models),
            ),
          ),
        if (state.notices.isNotEmpty)
          TvInlineNotice(
            key: const Key('notice-banner'),
            kind: TvNoticeKind.info,
            title: extra > 0 ? '${state.notices.last}  (+$extra more)' : state.notices.last,
            action: TvButton(
              'Dismiss',
              key: const Key('notice-dismiss'),
              dense: true,
              kind: TvButtonKind.ghost,
              onPressed: state.dismissNotice,
            ),
          ),
      ],
    );
  }
}
