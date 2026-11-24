import 'package:material_ui/material_ui.dart';

import '../colors.dart';
import '../sizes.dart';
import '../type.dart';

/// A page's title row: title (truncates with a tooltip), tag, one meta line
/// and its actions, the primary one last.
class TvPageHeader extends StatelessWidget {
  const TvPageHeader({super.key, required this.title, this.tag, this.meta, this.actions = const []});

  final String title;
  final Widget? tag;
  final String? meta;
  final List<Widget> actions;

  @override
  Widget build(BuildContext context) {
    final c = TvColors.of(context);
    return Padding(
      padding: const EdgeInsets.fromLTRB(TvSpace.s24, TvSpace.s16, TvSpace.s24, 0),
      child: Row(
        children: [
          Flexible(
            child: Tooltip(
              message: title,
              waitDuration: const Duration(milliseconds: 600),
              child: Text(
                title,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: TvType.pageTitle.copyWith(color: c.textPrimary),
              ),
            ),
          ),
          if (tag != null) ...[const SizedBox(width: 10), tag!],
          const Spacer(),
          if (meta != null) ...[
            Flexible(
              child: Text(
                meta!,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: TvType.helper.copyWith(color: c.textHelper),
              ),
            ),
            const SizedBox(width: TvSpace.s12),
          ],
          for (final (i, a) in actions.indexed) ...[if (i > 0) const SizedBox(width: TvSpace.s8), a],
        ],
      ),
    );
  }
}

/// A side panel's 36 px header: an upper-case label and its actions.
class TvPanelHeader extends StatelessWidget {
  const TvPanelHeader(this.label, {super.key, this.actions = const []});

  final String label;
  final List<Widget> actions;

  @override
  Widget build(BuildContext context) => SizedBox(
    height: TvSize.tabBar,
    child: Padding(
      padding: const EdgeInsets.only(left: TvSpace.s16, right: TvSpace.s8),
      child: Row(
        children: [
          Expanded(
            child: Text(
              label.toUpperCase(),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: TvType.label.copyWith(color: TvColors.of(context).textHelper),
            ),
          ),
          ...actions,
        ],
      ),
    ),
  );
}

/// The 38 px bar above content: what is open on the left, its actions right.
class TvToolbar extends StatelessWidget {
  const TvToolbar({super.key, this.leading, this.actions = const []});

  final Widget? leading;
  final List<Widget> actions;

  @override
  Widget build(BuildContext context) => Container(
    height: TvSize.toolbar,
    padding: const EdgeInsets.only(left: TvSpace.s24, right: TvSpace.s12),
    decoration: BoxDecoration(
      border: Border(bottom: BorderSide(color: TvColors.of(context).borderSubtle)),
    ),
    child: Row(
      children: [
        Expanded(
          child: DefaultTextStyle.merge(
            style: TvType.body.copyWith(color: TvColors.of(context).textPrimary),
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            child: leading ?? const SizedBox.shrink(),
          ),
        ),
        ...actions,
      ],
    ),
  );
}
