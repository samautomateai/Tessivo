import 'dart:async';

import 'package:material_ui/material_ui.dart';

import '../app_state.dart';
import '../design/colors.dart';
import '../design/components/tags.dart';
import '../design/icons.dart';
import '../design/sizes.dart';
import '../design/type.dart';
import '../widgets/approvals_sheet.dart';

/// 950, 412k, 2M, 1.3M.
String formatTokens(int n) {
  if (n < 1000) return '$n';
  if (n < 1000000) return '${(n / 1000).round()}k';
  final m = n / 1000000;
  return m == m.roundToDouble() ? '${m.round()}M' : '${m.toStringAsFixed(1)}M';
}

/// "Today 412k / 2M tokens", or without a daily limit "Today 412k tokens".
String usageText(int tokens, int dayLimit) => dayLimit > 0
    ? 'Today ${formatTokens(tokens)} / ${formatTokens(dayLimit)} tokens'
    : 'Today ${formatTokens(tokens)} tokens';

/// The 22 px bar at the bottom: core connection, default model, today's
/// tokens and approvals waiting. Items that lead somewhere are buttons.
class StatusBar extends StatelessWidget {
  const StatusBar({super.key, required this.state});

  final AppState state;

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    final text = TvType.helper.copyWith(color: c.statusBarText);
    final usage = state.usage;
    final waiting = state.approvals.length;
    return Container(
      key: const Key('status-bar'),
      height: TvSize.statusBar,
      padding: const EdgeInsets.symmetric(horizontal: 10),
      decoration: BoxDecoration(
        color: c.statusBar,
        border: identical(c, TvColors.highContrast) ? Border(top: BorderSide(color: c.borderStrong)) : null,
      ),
      child: DefaultTextStyle(
        style: text,
        maxLines: 1,
        overflow: TextOverflow.ellipsis,
        child: Row(
          children: [
            _Item(
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  TvStatusDot(state.connectionLost ? c.warning : c.success, size: 7),
                  const SizedBox(width: 6),
                  Text(state.connectionLost ? 'Reconnecting…' : 'Core connected'),
                ],
              ),
            ),
            if (state.defaultModel case final m? when m.isNotEmpty)
              Flexible(
                child: _Item(
                  onTap: () => state.openSettings(SettingsSection.models),
                  tooltip: 'Default model (Settings)',
                  child: Text(m),
                ),
              ),
            if (usage != null) Flexible(child: _Item(child: Text(usageText(usage.tokens, usage.dayLimit)))),
            const Spacer(),
            if (waiting > 0)
              Flexible(
                child: _Item(
                  tooltip: 'Open the approvals',
                  onTap: () => unawaited(showApprovalsSheet(context, state)),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      TvIconView(TvIcon.approvals, size: 12, color: c.statusBarText),
                      const SizedBox(width: 5),
                      Flexible(child: Text(waiting == 1 ? '1 approval waiting' : '$waiting approvals waiting')),
                    ],
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }
}

class _Item extends StatelessWidget {
  const _Item({required this.child, this.onTap, this.tooltip});

  final Widget child;
  final VoidCallback? onTap;
  final String? tooltip;

  @override
  Widget build(BuildContext context) {
    final item = Padding(padding: const EdgeInsets.symmetric(horizontal: 6), child: child);
    if (onTap == null) return item;
    return Tooltip(
      message: tooltip ?? '',
      child: InkWell(onTap: onTap, child: item),
    );
  }
}
