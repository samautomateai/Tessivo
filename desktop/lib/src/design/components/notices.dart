import 'package:material_ui/material_ui.dart';

import '../colors.dart';
import '../icons.dart';
import '../sizes.dart';
import '../type.dart';

enum TvNoticeKind { info, success, warning, error }

Color _kindColor(TvColors c, TvNoticeKind k) => switch (k) {
  TvNoticeKind.info => c.info,
  TvNoticeKind.success => c.success,
  TvNoticeKind.warning => c.warning,
  TvNoticeKind.error => c.error,
};

TvIcon _kindIcon(TvNoticeKind k) => switch (k) {
  TvNoticeKind.info => TvIcon.notifications,
  TvNoticeKind.success => TvIcon.success,
  TvNoticeKind.warning => TvIcon.warning,
  TvNoticeKind.error => TvIcon.error,
};

/// A Carbon inline notification: a 3 px coloured edge, icon, title, message
/// and an optional action, on the first layer.
class TvInlineNotice extends StatelessWidget {
  const TvInlineNotice({super.key, required this.kind, required this.title, this.message, this.action});

  final TvNoticeKind kind;
  final String title;
  final String? message;
  final Widget? action;

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    final tone = _kindColor(c, kind);
    return Container(
      padding: const EdgeInsets.fromLTRB(TvSpace.s12, 10, TvSpace.s12, 10),
      decoration: BoxDecoration(
        color: c.layer01,
        border: Border(left: BorderSide(color: tone, width: 3)),
      ),
      child: Row(
        children: [
          TvIconView(_kindIcon(kind), color: tone),
          const SizedBox(width: TvSpace.s8),
          Text(title, style: TvType.heading01.copyWith(color: c.textPrimary)),
          if (message != null) ...[
            const SizedBox(width: TvSpace.s8),
            Expanded(
              child: Text(message!, style: TvType.body.copyWith(color: c.textSecondary)),
            ),
          ] else
            const Spacer(),
          ?action,
        ],
      ),
    );
  }
}

/// A short message at the bottom of the window (replaces snack bars).
void showTvToast(BuildContext context, String message, {TvNoticeKind kind = TvNoticeKind.info}) {
  final c = TvColors.of(context);
  ScaffoldMessenger.of(context).showSnackBar(
    SnackBar(
      behavior: SnackBarBehavior.floating,
      width: 420,
      backgroundColor: c.layer02,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(TvRadius.medium),
        side: BorderSide(color: _kindColor(c, kind)),
      ),
      content: Row(
        children: [
          TvIconView(_kindIcon(kind), color: _kindColor(c, kind)),
          const SizedBox(width: TvSpace.s8),
          Expanded(
            child: Text(message, style: TvType.body.copyWith(color: c.textPrimary)),
          ),
        ],
      ),
    ),
  );
}
